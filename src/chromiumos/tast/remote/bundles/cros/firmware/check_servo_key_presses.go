// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"

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

	readKeys := make(chan bool)
	go func(readKeys chan bool) {
		// Start listening on the DUT for activity from the servo's keyboard emulator,
		// which the main goroutine will send.
		s.Log("Sending ReadServoKeyboard to the DUT")
		res, err := utils.ReadServoKeyboard(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Error during ReadServoKeyboard: ", err)
		}
		const logFileName = "raw_evdev_events.txt"
		logPath := filepath.Join(s.OutDir(), logFileName)
		if err := ioutil.WriteFile(logPath, []byte(res.Keys), 0644); err != nil {
			s.Error("Failed to save the keyboard output: ", err)
		}
		readKeys <- true
	}(readKeys)

	// TODO(kmshelton): Make utils.ReadServoKeyboard a streaming RPC that returns decoded key events,
	// so it can tell the test when it has started listening for keys, to avoid a race condition
	// here of the keys being sent before the utils service is listening, and to enable validation
	// of which keys where pressed.
	svo.KeypressWithDuration(ctx, servo.CtrlD, servo.DurTab)
	svo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab)
	<-readKeys
}
