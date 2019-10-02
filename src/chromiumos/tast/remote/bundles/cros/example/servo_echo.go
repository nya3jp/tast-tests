// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServoEcho,
		Desc:     "Demonstrates running a test using Servo",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"servo"},
	})
}

// ServoEcho demonstrates how you'd use Servo in a Tast test using the echo method.
func ServoEcho(ctx context.Context, s *testing.State) {
	dut, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	// This is expected to fail in VMs, since Servo is unusable there and the "servo" var won't
	// be supplied. https://crbug.com/967901 tracks finding a way to skip tests when needed.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	const msg = "hello from servo"
	s.Logf("Sending echo request for %q", msg)
	actualMessage, err := pxy.Servo().Echo(ctx, msg)
	if err != nil {
		s.Fatal("Got error: ", err)
	}
	s.Logf("Got response %q", actualMessage)
	const expectedMessage = "ECH0ING: " + msg
	if actualMessage != expectedMessage {
		s.Fatalf("Got message %q; expected %q", actualMessage, expectedMessage)
	}
}
