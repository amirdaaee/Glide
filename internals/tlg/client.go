package tlg

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

//go:generate mockgen -source=client.go -destination=../../mocks/tlg/client.go -package=mocks
type IClient interface {
	Connect() error
	GetClient() *gotgproto.Client
}

type client struct {
	sessCfg  *SessionConfig
	client   *gotgproto.Client
	token    string
	connLock sync.Mutex
	ll       *zap.Logger
}

var _ IClient = (*client)(nil)

func (tc *client) Connect() error {
	tc.connLock.Lock()
	defer tc.connLock.Unlock()
	ll := tc.ll.Named("Connect")
	if tc.client != nil {
		ll.Warn("client is already connected")
		return nil
	}
	ll.Info("connecting to tg")
	cl, err := tc.getTgClient()
	if err != nil {
		return fmt.Errorf("can not get tg client: %w", err)
	}
	tc.client = cl
	return nil
}
func (tc *client) GetClient() *gotgproto.Client {
	return tc.client
}
func (tc *client) getTgClient() (*gotgproto.Client, error) {
	ll := tc.ll.Named("getTgClient")
	sessCfg := tc.sessCfg
	if err := os.Mkdir(sessCfg.SessionDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("can not create session dir: %s", err)
	}
	ll.Sugar().Infof("session dir: %s", sessCfg.SessionDir)
	sessionDBPath := fmt.Sprintf("%s/worker-%s.sqlite3", sessCfg.SessionDir, strings.Split(tc.token, ":")[0])
	ll.Sugar().Infof("session db path: %s", sessionDBPath)
	sessionType := sessionMaker.SqlSession(sqlite.Open(sessionDBPath))
	clOpts := gotgproto.ClientOpts{
		Session:          sessionType,
		DisableCopyright: true,
		Middlewares:      tc.getMiddlewares(),
	}
	if resolver, err := sessCfg.getSocksDialer(); err != nil {
		ll.With(zap.Error(err)).Error("can not get socks dialer. using default")
	} else if resolver != nil {
		ll.Sugar().Infof("using socks dialer")
		clOpts.Resolver = *resolver
	}
	client, err := gotgproto.NewClient(
		sessCfg.AppID,
		sessCfg.AppHash,
		gotgproto.ClientTypeBot(tc.token),
		&clOpts,
	)
	if err != nil {
		return nil, fmt.Errorf("can not create gotgproto client: %w", err)
	}
	return client, nil
}

func (tc *client) getMiddlewares() []telegram.Middleware {
	return []telegram.Middleware{
		floodwait.NewSimpleWaiter().WithMaxRetries(10).WithMaxWait(5 * time.Second),
		ratelimit.New(rate.Every(time.Millisecond*100), 5),
	}
}
func NewTgClient(sessCfg *SessionConfig, token string) IClient {
	return &client{
		sessCfg: sessCfg,
		token:   token,
	}
}
