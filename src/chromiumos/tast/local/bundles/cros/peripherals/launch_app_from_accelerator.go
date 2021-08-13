// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// accelTestParams contains all the data needed to run a single test iteration.
type accelTestParams struct {
	app         apps.App
	keystroke   string
	featureFlag string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchAppFromAccelerator,
		Desc: "Peripherals app can be found and launched with an accelerator",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val: accelTestParams{
					app:         apps.Diagnostics,
					keystroke:   "Ctrl+Search+Esc",
					featureFlag: "DiagnosticsApp",
				},
			},
		},
	})
}

// LaunchAppFromAccelerator verifies launching an app with an accelerator.
func LaunchAppFromAccelerator(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(accelTestParams)

	cr, err := chrome.New(ctx, chrome.EnableFeatures(params.featureFlag))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx) // Close our own chrome instance.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard instance: ", err)
	}
	defer kb.Close()

	// Briefly sleep so that the device will be ready to process the input.
	// TODO(joonbug): Find a suitable polling target for this instead of sleep.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Error stablizing: ", err)
	}

	s.Logf("Sending keystroke: %s", params.keystroke)
	err = kb.Accel(ctx, params.keystroke)
	if err != nil {
		s.Fatal("Failed to search and launch app: ", err)
	}

	err = ash.WaitForApp(ctx, tconn, params.app.ID, time.Minute)
	if err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}
}
