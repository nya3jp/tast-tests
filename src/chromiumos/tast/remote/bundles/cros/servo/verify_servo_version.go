// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VerifyServoVersion,
		Desc: "Demonstrates running a test using Servo.",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr: []string{"informational"},
	})
}

// Verify Servo version. Ensures Servo and Servod are set up properly.
func VerifyServoVersion(ctx context.Context, s *testing.State) {
	method := "get_version"
	response, err := servo.Call(method, s)

	if err != nil {
		s.Fatal("XML-RPC error: ", err)
		return
	}

	if response.Message != "servo_v4" {
		s.Fatal("Version is not 'servo_v4': ", response.Message)
	}
}
