package serial

import (
	"context"

	tarm "github.com/tarm/serial"
)

// ConnectedPort shares the interface with RemotePort, delegates to tarm.serial.
type ConnectedPort struct {
	port *tarm.Port
}

func (p *ConnectedPort) Read(ctx context.Context, b []byte) (n int, err error) {
	return p.port.Read(b)
}

func (p *ConnectedPort) Write(ctx context.Context, b []byte) (n int, err error) {
	return p.port.Write(b)
}

func (p *ConnectedPort) Flush(ctx context.Context) error {
	return p.port.Flush()
}

func (p *ConnectedPort) Close(ctx context.Context) error {
	return p.port.Close()
}
