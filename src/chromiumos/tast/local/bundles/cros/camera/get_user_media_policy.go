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
	"chromiumos/tast/local/bundles/cros/camera/getusermedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetUserMediaPolicy,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies that admin policy can successfully ban getUserMedia",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera", "informational"},
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome"},
		Data:         append(getusermedia.DataFiles(), "getusermedia.html"),
		Params: []testing.Param{
			{
				Name:    "ash",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val:     browser.TypeAsh,
			},
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
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

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

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
