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
		Func:     Servo,
		Desc:     "Demonstrates running a test using Servo",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"informational"},
	})
}

// Servo sends an echo request to servod.
func Servo(ctx context.Context, s *testing.State) {

	// TODO(jeffcarp): Parameterize servod host and port.
	svo, err := servo.New(ctx, "localhost", 9999)
	if err != nil {
		s.Fatal("Servo init error: ", err)
	}
	args := servo.Args{"hello from servo"}
	response, err := svo.Call("echo", args)
	if err != nil {
		s.Fatal("XML-RPC error: ", err)
	}

	const expectedMessage = "ECH0ING: hello from servo"
	if response.Message != expectedMessage {
		s.Fatalf("Got message %q; expected %q", response.Message, expectedMessage)
	}
}
