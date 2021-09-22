// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	commonSerial "chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/remote/firmware/serial"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    RemoteSerialPort,
		Desc:    "Test RemoteSerialPort",
		Timeout: 1 * time.Minute,
		Contacts: []string{
			"aluo@chromium.org",            // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		ServiceDeps: []string{"tast.cros.firmware.SerialPortService"},
		Attr:        []string{"group:firmware", "firmware_experimental"},
	})
}

func RemoteSerialPort(ctx context.Context, s *testing.State) {
	rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Error dialing rpc: ", err)
	}
	defer rpcClient.Close(ctx)

	pty1, pty2, cancel, done, err := commonSerial.CreatePtyPair(ctx, s.DUT())
	if err != nil {
		s.Fatal("Error creating pty: ", err)
	}
	defer func() {
		cancel()
		<-done
	}()
	s.Logf("Created ptys: %s %s", pty1, pty2)

	serviceClient := pb.NewSerialPortServiceClient(rpcClient.Conn)
	o1 := serial.NewRemotePortOpener(serviceClient, pty1, 115200, 50*time.Millisecond)
	o2 := serial.NewRemotePortOpener(serviceClient, pty2, 115200, 50*time.Millisecond)

	err = commonSerial.DoTestPort(ctx, s.Log, o1, o2)

	if err != nil {
		s.Fatal("Test failed: ", err)
	}
}
