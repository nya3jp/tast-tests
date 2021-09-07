// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMediaPolicy,
		Desc:         "Verifies that admin policy can successfully ban getUserMedia",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
		Params: []testing.Param{
			{
				Name:    "ash",
				Fixture: "chromePolicyLoggedIn",
				Val:     lacros.ChromeTypeChromeOS,
			},
			{
				Name:              "lacros",
				Fixture:           "lacrosPolicyLoggedIn",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               lacros.ChromeTypeLacros,
			},
		},
	})
}

// GetUserMediaPolicy tests whether admin policy can successfully blocks getUserMedia().
func GetUserMediaPolicy(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var fdms *fakedms.FakeDMS
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		cr = s.FixtValue().(launcher.FixtValueImpl).Chrome
		fdms = s.FixtValue().(launcher.FixtValueImpl).FakeDMS
	} else {
		cr = s.FixtValue().(*fixtures.FixtData).Chrome
		fdms = s.FixtValue().(*fixtures.FixtData).FakeDMS
	}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policy.VideoCaptureAllowed{Val: false}}); err != nil {
		s.Fatal("Failed to serve policy to ban video capture: ", err)
	}

	var conn *chrome.Conn
	var err error
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		lacros, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtValueImpl))
		if err != nil {
			s.Fatal("Failed to launch lacros-chrome: ", err)
		}
		defer lacros.Close(cleanupCtx)

		if conn, err = lacros.NewConn(ctx, "chrome://newtab"); err != nil {
			s.Fatal("Failed to create a tab in Chrome: ", err)
		}
	} else {
		if conn, err = cr.NewConn(ctx, "chrome://newtab"); err != nil {
			s.Fatal("Failed to create a tab in Chrome: ", err)
		}
	}

	if err := conn.Call(ctx, nil, `async () => {
		return navigator.mediaDevices.getUserMedia({video: true});
	}`); err == nil { // It is doesn't fail, it is unexpected.
		s.Fatal("Failed to ban getUserMedia() by the policy")
	} else if err.Error() != "DOMException: Permission denied" {
		s.Fatal("Unexpected error when calling getUserMedia(): ", err)
	}
}
