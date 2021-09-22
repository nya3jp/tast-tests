// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
	"time"

	common "chromiumos/tast/common/firmware/serial"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// RemotePortOpener holds data needed to open a RemotePort.
type RemotePortOpener struct {
	common.Config
	serviceClient pb.SerialPortServiceClient
}

// OpenPort opens and returns the port.
func (c *RemotePortOpener) OpenPort(ctx context.Context) (common.Port, error) {
	portCfg := pb.SerialPortConfig{
		Name:        c.Name,
		Baud:        int64(c.Baud),
		ReadTimeout: int64(c.ReadTimeout),
	}

	testing.ContextLog(ctx, "RemotePortOpener Opening port")
	id, err := c.serviceClient.Open(ctx, &portCfg)
	if err != nil {
		testing.ContextLog(ctx, "RemotePortOpener Opening port failed: ", err)
		return nil, err
	}
	testing.ContextLog(ctx, "Opening port success")

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
	cfg := common.Config{name, baud, readTimeout}
	return &RemotePortOpener{
		Config:        cfg,
		serviceClient: client,
	}
}
