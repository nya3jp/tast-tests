// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VerifyServoVersion,
		Desc: "Demonstrates running a test using Servo",
		Contacts: []string{"jeffcarp@chromium.org", "derat@chromium.org", "tast-users@chromium.org"},
		Attr: []string{"informational"},
	})
}

// Verify Servo version. Ensures Servo and Servod are set up properly.
func VerifyServoVersion(ctx context.Context, s *testing.State) {
	const versionMustContain = "servo"

	response, err := servo.Call("get_version")
	if err != nil {
		s.Fatal("XML-RPC error: ", err)
	}

	if !strings.Contains(response.Message, versionMustContain) {
		s.Fatalf("Got version %q; must contain %q", response.Message, versionMustContain)
	}
}
