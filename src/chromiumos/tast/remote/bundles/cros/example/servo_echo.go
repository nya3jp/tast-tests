// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ServoEcho,
		Desc:     "Demonstrates running a test using Servo",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"disabled", "informational"},
	})
}

// ServoEcho demonstrates how you'd use Servo in a Tast test using the echo method.
func ServoEcho(ctx context.Context, s *testing.State) {
	// TODO(jeffcarp): Parameterize servod host and port.
	const msg = "hello from servo"
	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}

	actualMessage, err := svo.Echo(ctx, "hello from servo")
	if err != nil {
		s.Fatal("Got error: ", err)
	}
	const expectedMessage = "ECH0ING: " + msg
	if actualMessage != expectedMessage {
		s.Fatalf("Got message %q; expected %q", actualMessage, expectedMessage)
	}
}
