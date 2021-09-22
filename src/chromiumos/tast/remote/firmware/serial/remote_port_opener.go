// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"

	common "chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
)

// RemotePortOpener holds data needed to open a RemotePort.
type RemotePortOpener struct {
	config        common.Config
	serviceClient pb.SerialPortServiceClient
}

// OpenPort opens and returns the port.
func (c *RemotePortOpener) OpenPort(ctx context.Context) (common.Port, error) {
	id, err := c.serviceClient.Open(ctx, &pb.SerialPortConfig{
		Name:        c.config.Name,
		Baud:        int64(c.config.Baud),
		ReadTimeout: ptypes.DurationProto(c.config.ReadTimeout),
	})
	if err != nil {
		return nil, errors.Wrap(err, "opening remote port")
	}

	return &RemotePort{c.serviceClient, id.GetValue()}, nil
}

// NewRemotePortOpener creates a RemotePortOpener.
//
// Example:
//   rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
//   defer rpcClient.Close(ctx)
//
//   if err != nil {
//       s.Fatal("rpcDial: ", err)
//   }
//   defer rpcClient.Close(ctx)
//
//   serviceClient := pb.NewSerialPortServiceClient(rpcClient.Conn)
//
//   opener := NewRemotePortOpener(serviceClient, "/path/to/device", 115200, 2 * time.Second)
//   port := opener.OpenPort(ctx)
func NewRemotePortOpener(client pb.SerialPortServiceClient, name string, baud int, readTimeout time.Duration) *RemotePortOpener {
	return &RemotePortOpener{
		config:        common.Config{Name: name, Baud: baud, ReadTimeout: readTimeout},
		serviceClient: client,
	}
}
