// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CheckServoKeyPresses,
		Desc:        "Verifies that key presses can be initiated on the servo's keyboard emulator and that the DUT can receive them",
		Contacts:    []string{"kmshelton@chromium.org", "cros-fw-engprod@google.com", "chromeos-firmware@google.com"},
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Attr:        []string{"group:mainline", "informational"},
		Vars:        []string{"servo"},
	})
}

func CheckServoKeyPresses(ctx context.Context, s *testing.State) {
	var wg sync.WaitGroup
	dut := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	svo := pxy.Servo()

	const msg = "hello from servo"
	s.Logf("Sending echo request for %q", msg)
	actualMessage, err := svo.Echo(ctx, msg)
	if err != nil {
		s.Fatal("Got error: ", err)
	}
	s.Logf("Got response %q", actualMessage)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		res, err := utils.ReadServoKeyboard(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Error during ReadServoKeyboard: ", err)
		}
		s.Log("Read the following from the servo key emulator:", res.Keys)
	}(&wg)

	// TODO(kmshelton): Make the RPC a streaming RPC, so it can tell the
	// test when it has started listening for keys, to avoid this sleep that
	// is intended to wait until the utils service is listening.
	testing.Sleep(ctx, 2*time.Second)
	svo.KeypressWithDuration(ctx, servo.CtrlD, servo.DurTab)
	svo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab)
}
