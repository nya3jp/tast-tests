// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"reflect"
	"time"

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
	// The value of durationToListenForPressesOnDut is in seconds and is experimentally determined.
	const durationToListenForPressesOnDut uint32 = 5
	const firstExpectedKey = "KEY_ENTER"
	const secondExpectedKey = "KEY_ENTER"

	dut := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
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
		res, err := utils.ReadServoKeyboard(ctx, &fwpb.ReadServoKeyboardRequest{Duration: durationToListenForPressesOnDut})
		if err != nil {
			s.Fatal("Error during ReadServoKeyboard: ", err)
		}
		s.Log("Keys that were read, ", res.Key)
		expectedKeys := []string{firstExpectedKey, secondExpectedKey}
		if !reflect.DeepEqual(expectedKeys, res.Key) {
			s.Errorf("Something failed in the keys that were read; want %v, got %v", expectedKeys, res.Key)
		}
	}()

	// TODO(kmshelton): Make utils.ReadServoKeyboard a streaming RPC that returns decoded key events,
	// so it can tell the test when it has started listening for keys, to avoid a race condition
	// here of the keys being sent before the utils service is listening.
	testing.Sleep(ctx, 1*time.Second)
	svo.KeypressWithDuration(ctx, servo.USBEnter, servo.DurTab)
	svo.KeypressWithDuration(ctx, servo.USBEnter, servo.DurTab)
	<-readKeys
}
