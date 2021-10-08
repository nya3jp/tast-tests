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
		Attr:        []string{"group:firmware", "firmware_unstable"},
	})
}

func RemoteSerialPort(ctx context.Context, s *testing.State) {
	rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Error dialing rpc: ", err)
	}
	defer rpcClient.Close(ctx)

	pty1, pty2, cancel, done, err := commonSerial.CreateDUTPTYPair(ctx, s.DUT())
	if err != nil {
		s.Fatal("Error creating pty: ", err)
	}
	defer func() {
		cancel()
		<-done
	}()
	s.Logf("Created ptys: %s %s", pty1, pty2)

	serviceClient := pb.NewSerialPortServiceClient(rpcClient.Conn)
	o1 := serial.NewRemotePortOpener(serviceClient, pty1, 115200, 200*time.Millisecond)
	o2 := serial.NewRemotePortOpener(serviceClient, pty2, 115200, 200*time.Millisecond)

	s.Log("Opening remote ports should work")
	p1, err := o1.OpenPort(ctx)
	if err != nil {
		s.Fatal("Open port 1: ", err)
	}
	defer p1.Close(ctx)

	p2, err := o2.OpenPort(ctx)
	if err != nil {
		s.Fatal("Open port 2: ", err)
	}
	defer p2.Close(ctx)

	if err = commonSerial.DoTestRead(ctx, s.Log, p1, p2); err != nil {
		s.Fatal("TestRead failed: ", err)
	}

	if err = commonSerial.DoTestWrite(ctx, s.Log, p1, p2); err != nil {
		s.Fatal("TestWrite failed: ", err)
	}

	if err = commonSerial.DoTestFlush(ctx, s.Log, p1, p2); err != nil {
		s.Fatal("TestFlush failed: ", err)
	}
}
