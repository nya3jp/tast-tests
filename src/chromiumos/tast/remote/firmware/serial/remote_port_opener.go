package serial

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/testing"
	common "chromiumos/tast/common/firmware/serial"
	pb "chromiumos/tast/services/cros/firmware"
)

// RemotePortOpener facilitates lazy-opening of the serial port.
type RemotePortOpener struct {
	common.Config
	rpcConn *grpc.ClientConn
}

// OpenPort opens and returns the port.
func (c *RemotePortOpener) OpenPort(ctx context.Context) (common.Port, error) {
	portClient := pb.NewSerialPortServiceClient(c.rpcConn)
	portCfg := pb.SerialPortConfig{
		Name: c.Name,
		Baud: int64(c.Baud),
		ReadTimeout: int64(c.ReadTimeout),
	}

	testing.ContextLog(ctx, "RemotePortOpener Opening port")
	if _, err := portClient.Open(ctx, &portCfg); err != nil {
	        testing.ContextLog(ctx, "RemotePortOpener Opening port failed: ", err)
		return nil, err
	}
	testing.ContextLog(ctx, "Opening port success")

        return &RemotePort{portClient}, nil
}

// NewRemotePortOpener creates a RemotePortOpener.
//
// Example:
//   TODO
func NewRemotePortOpener(rpcConn *grpc.ClientConn, name string, baud int, readTimeout time.Duration) *RemotePortOpener {
	cfg := common.Config{name, baud, readTimeout}
	return &RemotePortOpener{
	Config: cfg,
	rpcConn: rpcConn,
	}
}
