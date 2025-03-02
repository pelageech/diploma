package healthcheck

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
)

type HTTPCheck struct {
	Url    *url.URL
	Client *http.Client
	OnDone func()
}

func (c *HTTPCheck) Run(ctx context.Context) error {
	resp, err := c.Client.Get(c.Url.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

type TCPCheck struct {
	Dialer net.Dialer
	Addr   string
	OnDone func()
}

func NewTCPCheck(addr string) *TCPCheck {
	a := &atomic.Int64{}
	go func() {
		slog.Info("now", "i", a.Load())
	}()
	return &TCPCheck{
		Dialer: net.Dialer{},
		Addr:   addr,
		OnDone: func() {},
	}
}

func (c *TCPCheck) Run(ctx context.Context) error {
	conn, err := c.Dialer.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return err
	}
	defer c.OnDone()
	defer conn.Close()
	return nil
}
