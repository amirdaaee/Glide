package tlg

import (
	"fmt"
	"net/url"

	"github.com/amirdaaee/Glide/internals/log"
	"github.com/gotd/td/telegram/dcs"
	"golang.org/x/net/proxy"
)

// SessionConfig holds Telegram app credentials, session dir, and optional SOCKS proxy.
type SessionConfig struct {
	SocksProxy string
	SessionDir string
	AppID      int
	AppHash    string
}

// getSocksDialer returns a DC resolver for SocksProxy, or nil if unset.
func (sessCfg *SessionConfig) getSocksDialer() (*dcs.Resolver, error) {
	ll := log.Named(log.TELEGRAM, "SessionConfig")
	proxyUriStr := sessCfg.SocksProxy
	if proxyUriStr == "" {
		ll.Info("no socks proxy provided")
		return nil, nil
	}
	proxyUri, err := url.Parse(proxyUriStr)
	if err != nil {
		return nil, fmt.Errorf("can not parse proxy url (%s): %s", proxyUriStr, err)
	}
	uPass, _ := proxyUri.User.Password()
	sock5, err := proxy.SOCKS5("tcp", proxyUri.Host, &proxy.Auth{
		User:     proxyUri.User.Username(),
		Password: uPass,
	}, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("can not create socks proxy (%s): %s", proxyUriStr, err)
	}
	dc := sock5.(proxy.ContextDialer)
	dialler := dcs.Plain(dcs.PlainOptions{
		Dial: dc.DialContext,
	})
	ll.Info("socks dialer created")
	return &dialler, nil
}
