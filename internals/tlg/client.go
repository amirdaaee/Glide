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

// IClient is a single-use Telegram client. After RunBot or StartClient returns
// (or the background run ends), the instance must not be started again.
type IClient interface {
	// RunBot authenticates as a bot and blocks until ctx is cancelled.
	RunBot(ctx context.Context) error
	// StartClient runs the client in the background and returns once authenticated.
	StartClient(ctx context.Context) error
	// API returns the MTProto API client, or nil before ready and after the run exits.
	API() *tg.Client
	SetUpdateHandler(h telegram.UpdateHandler)
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
	ready         *clientReady
	runMu         sync.Mutex
	isRunning     bool
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

func (tc *client) RunBot(ctx context.Context) error {
	return tc.doRun(ctx)
}

func (tc *client) StartClient(ctx context.Context) error {
	go func() {
		if err := tc.doRun(ctx); err != nil && ctx.Err() == nil {
			tc.ll.With(zap.Error(err)).Error("client run ended with error")
		}
	}()
	select {
	case <-tc.ready.ready:
		return tc.ready.ReadyErr()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (tc *client) setAPI(api *tg.Client) {
	tc.apiMu.Lock()
	tc.api = api
	tc.apiMu.Unlock()
}

func (tc *client) doRun(ctx context.Context) error {
	tc.runMu.Lock()
	if tc.isRunning {
		tc.runMu.Unlock()
		return fmt.Errorf("client is already running; do not reuse client instance")
	}
	tc.isRunning = true
	tc.runMu.Unlock()
	return tc.tg.Run(ctx, func(ctx context.Context) error {
		ll := tc.ll.Named("Run")
		if err := tc.authorize(ctx); err != nil {
			tc.ready.notifyReady(err)
			return fmt.Errorf("can not authorize: %w", err)
		}
		tc.setAPI(tc.tg.API())
		tc.ready.notifyReady(nil)
		ll.Info("client ready")
		<-ctx.Done()
		return ctx.Err()
	})
}
func (tc *client) authorize(ctx context.Context) error {
	ll := tc.ll.Named("Authorize")
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
	return nil
}
func (tc *client) buildTelegramClient() (*telegram.Client, error) {
	sessCfg := tc.sessCfg
	if err := os.MkdirAll(sessCfg.SessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("can not create session dir: %w", err)
	}
	sessionPath := fmt.Sprintf("%s/%s-%s.json", sessCfg.SessionDir, tc.sessionPrefix, strings.Split(tc.token, ":")[0])
	tc.ll.Info("session path", zap.String("path", sessionPath))

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
		ready:         &clientReady{ready: make(chan struct{}), readyErr: nil, readyMu: sync.RWMutex{}},
	}
	cl, err := tc.buildTelegramClient()
	if err != nil {
		return nil, err
	}
	tc.tg = cl
	return tc, nil
}

// ===
type clientReady struct {
	ready    chan struct{}
	readyErr error
	readyMu  sync.RWMutex
}

func (cr *clientReady) ReadyErr() error {
	cr.readyMu.RLock()
	defer cr.readyMu.RUnlock()
	return cr.readyErr
}
func (cr *clientReady) notifyReady(err error) {
	cr.readyMu.Lock()
	cr.readyErr = err
	cr.readyMu.Unlock()
	select {
	case <-cr.ready:
	default:
		close(cr.ready)
	}
}
