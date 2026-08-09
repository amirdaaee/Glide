package tlg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

//go:generate mockgen -source=client.go -destination=../../mocks/tlg/client.go -package=mocks
type IClient interface {
	// RunBot authenticates as a bot and blocks until ctx is cancelled.
	RunBot(ctx context.Context) error
	// StartClient runs the client in the background and returns once authenticated.
	StartClient(ctx context.Context) error
	// API is valid only while the client is running.
	API() *tg.Client
	SetUpdateHandler(h telegram.UpdateHandler)
}

type updateHandlerHolder struct {
	mu sync.RWMutex
	h  telegram.UpdateHandler
}

func (h *updateHandlerHolder) Handle(ctx context.Context, u tg.UpdatesClass) error {
	h.mu.RLock()
	handler := h.h
	h.mu.RUnlock()
	if handler == nil {
		return nil
	}
	return handler.Handle(ctx, u)
}

func (h *updateHandlerHolder) set(handler telegram.UpdateHandler) {
	h.mu.Lock()
	h.h = handler
	h.mu.Unlock()
}

type client struct {
	sessCfg       *SessionConfig
	tg            *telegram.Client
	token         string
	sessionPrefix string
	api           *tg.Client
	apiMu         sync.RWMutex
	handler       *updateHandlerHolder
	ll            *zap.Logger

	runOnce  sync.Once
	ready    chan struct{}
	readyErr error
	readyMu  sync.Mutex
}

var _ IClient = (*client)(nil)

func (tc *client) SetUpdateHandler(h telegram.UpdateHandler) {
	tc.handler.set(h)
}

func (tc *client) API() *tg.Client {
	tc.apiMu.RLock()
	defer tc.apiMu.RUnlock()
	return tc.api
}

func (tc *client) setAPI(api *tg.Client) {
	tc.apiMu.Lock()
	tc.api = api
	tc.apiMu.Unlock()
}

func (tc *client) notifyReady(err error) {
	tc.readyMu.Lock()
	tc.readyErr = err
	tc.readyMu.Unlock()
	select {
	case <-tc.ready:
	default:
		close(tc.ready)
	}
}

func (tc *client) ReadyErr() error {
	tc.readyMu.Lock()
	defer tc.readyMu.Unlock()
	return tc.readyErr
}

func (tc *client) RunBot(ctx context.Context) error {
	return tc.doRun(ctx)
}

func (tc *client) StartClient(ctx context.Context) error {
	go func() {
		if err := tc.doRun(ctx); err != nil && ctx.Err() == nil {
			tc.ll.With(zap.Error(err)).Error("client run ended with error")
			tc.notifyReady(err)
		}
	}()
	select {
	case <-tc.ready:
		return tc.ReadyErr()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (tc *client) doRun(ctx context.Context) error {
	var runErr error
	tc.runOnce.Do(func() {
		runErr = tc.tg.Run(ctx, func(ctx context.Context) error {
			ll := tc.ll.Named("Run")
			status, err := tc.tg.Auth().Status(ctx)
			if err != nil {
				return fmt.Errorf("can not get auth status: %w", err)
			}
			if !status.Authorized {
				ll.Info("authorizing as bot")
				if _, err := tc.tg.Auth().Bot(ctx, tc.token); err != nil {
					return fmt.Errorf("can not auth as bot: %w", err)
				}
			} else {
				ll.Info("already authorized")
			}
			tc.setAPI(tc.tg.API())
			tc.notifyReady(nil)
			ll.Info("client ready")
			<-ctx.Done()
			return ctx.Err()
		})
	})
	return runErr
}

func (tc *client) buildTelegramClient() (*telegram.Client, error) {
	sessCfg := tc.sessCfg
	if err := os.MkdirAll(sessCfg.SessionDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("can not create session dir: %w", err)
	}
	sessionPrefix := tc.sessionPrefix
	if sessionPrefix == "" {
		sessionPrefix = "worker"
	}
	sessionPath := fmt.Sprintf("%s/%s-%s.json", sessCfg.SessionDir, sessionPrefix, strings.Split(tc.token, ":")[0])
	tc.ll.Sugar().Infof("session path: %s", sessionPath)

	opts := telegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
		Middlewares:    tc.getMiddlewares(),
		UpdateHandler:  tc.handler,
	}
	if resolver, err := sessCfg.getSocksDialer(); err != nil {
		tc.ll.With(zap.Error(err)).Error("can not get socks dialer. using default")
	} else if resolver != nil {
		tc.ll.Info("using socks dialer")
		opts.Resolver = *resolver
	}

	return telegram.NewClient(sessCfg.AppID, sessCfg.AppHash, opts), nil
}

func (tc *client) getMiddlewares() []telegram.Middleware {
	return []telegram.Middleware{
		floodwait.NewSimpleWaiter().WithMaxRetries(10).WithMaxWait(5 * time.Second),
		ratelimit.New(rate.Every(time.Millisecond*100), 5),
	}
}

func NewTgClient(sessCfg *SessionConfig, token, sessionPrefix string) (IClient, error) {
	if sessionPrefix == "" {
		sessionPrefix = "worker"
	}
	tc := &client{
		sessCfg:       sessCfg,
		token:         token,
		sessionPrefix: sessionPrefix,
		handler:       &updateHandlerHolder{},
		ll:            log.GetLogger(log.TELEGRAM),
		ready:         make(chan struct{}),
	}
	cl, err := tc.buildTelegramClient()
	if err != nil {
		return nil, err
	}
	tc.tg = cl
	return tc, nil
}

// BareChannelID converts a Bot API-style channel id (-100…) to an MTProto channel id.
func BareChannelID(id int64) int64 {
	if id >= 0 {
		return id
	}
	id = -id
	const prefix = int64(1_000_000_000_000) // 10^12
	if id >= prefix {
		return id % prefix
	}
	return id
}

// ChannelInputPeer builds an InputPeerChannel from config values.
func ChannelInputPeer(channelID, accessHash int64) *tg.InputPeerChannel {
	return &tg.InputPeerChannel{
		ChannelID:  BareChannelID(channelID),
		AccessHash: accessHash,
	}
}

// ChannelInput builds an InputChannel from config values.
func ChannelInput(channelID, accessHash int64) *tg.InputChannel {
	return &tg.InputChannel{
		ChannelID:  BareChannelID(channelID),
		AccessHash: accessHash,
	}
}
