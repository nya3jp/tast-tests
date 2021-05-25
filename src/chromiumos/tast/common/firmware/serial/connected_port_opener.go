package serial

import (
	"context"
	"time"

	tarm "github.com/tarm/serial"
)

type Config struct {
	Name string
	Baud int
	ReadTimeout time.Duration
}

type ConnectedPortOpener struct {
	Config
}

func (c *ConnectedPortOpener) OpenPort(ctx context.Context) (Port, error) {
        tarmCfg := &tarm.Config{Name: c.Name, Baud: c.Baud, ReadTimeout: c.ReadTimeout}

        p, err := tarm.OpenPort(tarmCfg)
        if err != nil {
                return nil, err
        }
        return &ConnectedPort{p}, nil
}

func NewConnectedPortOpener(name string, baud int, readTimeout time.Duration) *ConnectedPortOpener {
	cfg := Config{name, baud, readTimeout}
	return &ConnectedPortOpener{Config: cfg}
}
