// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CheckServoKeyPresses,
		Desc:        "Verifies that key presses can be initiated on the servo's keyboard emulator and that the DUT can receive and decode them",
		Contacts:    []string{"kmshelton@chromium.org", "cros-fw-engprod@google.com", "chromeos-firmware@google.com"},
		ServiceDeps: []string{"tast.cros.firmware.UtilsService"},
		Vars:        []string{"servo"},
	})
}

func CheckServoKeyPresses(ctx context.Context, s *testing.State) {
	// The value of listenSecs is in seconds and is experimentally determined.
	const listenSecs uint32 = 5
	const enterKey = "ENTER"

	dut := s.DUT()

	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	svo := pxy.Servo()

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	readKeys := make(chan struct{})
	go func() {
		defer close(readKeys)
		// Start listening on the DUT for activity from the servo's keyboard emulator,
		// which the main goroutine will send.
		s.Log("Sending ReadServoKeyboard to the DUT")
		res, err := utils.ReadServoKeyboard(ctx, &fwpb.ReadServoKeyboardRequest{Duration: listenSecs})
		if err != nil {
			s.Fatal("Error during ReadServoKeyboard: ", err)
		}
		expectedKeys := []string{enterKey, enterKey}
		if !cmp.Equal(res.Keys, expectedKeys) {
			s.Errorf("Something failed in the keys that were read; got %v, want %v", res.Keys, expectedKeys)
		}
	}()

	// TODO(kmshelton): Make utils.ReadServoKeyboard a streaming RPC that returns decoded key events,
	// so it can tell the test when it has started listening for keys, to avoid a race condition
	// here of the keys being sent before the utils service is listening.  We must sleep for now to
	// allow for the key press listening to get setup on the DUT.
	testing.Sleep(ctx, 1*time.Second)
	svo.KeypressWithDuration(ctx, servo.USBEnter, servo.DurTab)
	svo.KeypressWithDuration(ctx, servo.USBEnter, servo.DurTab)
	<-readKeys
}
