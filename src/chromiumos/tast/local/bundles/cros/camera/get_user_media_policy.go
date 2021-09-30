// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/lacrospolicyutil"
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
				Fixture: fixture.ChromePolicyLoggedIn,
				Val:     lacros.ChromeTypeChromeOS,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               lacros.ChromeTypeLacros,
			},
		},
	})
}

// GetUserMediaPolicy tests whether admin policy can successfully blocks getUserMedia().
func GetUserMediaPolicy(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{&policy.VideoCaptureAllowed{Val: false}}); err != nil {
		s.Fatal("Failed to serve policy to ban video capture: ", err)
	}

	// Setup browser based on the chrome type.
	br, cleanup, err := lacrospolicyutil.BrowserSetup(ctx, s.FixtValue(), s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer cleanup(cleanupCtx)

	// Run actual test.
	conn, err := br.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to connect to the browser: ", err)
	}
	defer conn.Close()

	if err := conn.Call(ctx, nil, `async () => {
		return navigator.mediaDevices.getUserMedia({video: true});
	}`); err == nil { // It is doesn't fail, it is unexpected.
		s.Fatal("Failed to ban getUserMedia() by the policy")
	} else if err.Error() != "DOMException: Permission denied" {
		s.Fatal("Unexpected error when calling getUserMedia(): ", err)
	}
}
