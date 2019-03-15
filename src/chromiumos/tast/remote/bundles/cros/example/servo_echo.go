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
	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}

	reply, err := svo.Echo(ctx, servo.EchoRequest{"hello from servo"})
	const expectedMessage = "ECH0ING: hello from servo"
	if reply.Message != expectedMessage {
		s.Fatalf("Got message %q; expected %q", reply.Message, expectedMessage)
	}
}
