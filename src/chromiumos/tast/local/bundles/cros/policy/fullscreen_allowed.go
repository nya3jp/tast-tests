// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FullscreenAllowed,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Behavior of FullscreenAllowed policy: checking if fullscreen is allowed or not",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
	})
}

// FullscreenAllowed tests the FullscreenAllowed policy.
func FullscreenAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		name                  string
		value                 *policy.FullscreenAllowed
		wantFullscreenEnabled bool
	}{
		{
			name:                  "unset",
			value:                 &policy.FullscreenAllowed{Stat: policy.StatusUnset},
			wantFullscreenEnabled: true,
		},
		{
			name:                  "disabled",
			value:                 &policy.FullscreenAllowed{Val: false},
			wantFullscreenEnabled: false,
		},
		{
			name:                  "enabled",
			value:                 &policy.FullscreenAllowed{Val: true},
			wantFullscreenEnabled: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open browser window.
			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to open browser window: ", err)
			}
			defer conn.Close()

			var isFullScreen bool
			if err := conn.Eval(ctx, `window.innerHeight == screen.availHeight`, &isFullScreen); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}
			// Check that the screen is not in full screen mode currently.
			if isFullScreen == true {
				s.Error("Browser should be not be in full screen mode initially")
			}

			// Define keyboard to type keyboard shortcut.
			kb, err := input.Keyboard(ctx)
			if err != nil {
				s.Fatal("Failed to get the keyboard: ", err)
			}
			defer kb.Close()

			// Type the shortcut to enter full screen mode.
			if err := kb.Accel(ctx, "F11"); err != nil {
				s.Fatal("Failed to type fullscreen hotkey: ", err)
			}

			// Wait for full screen to happen.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Check whether the browser is currently in full screen mode.
				if err := conn.Eval(ctx, `window.innerHeight == screen.availHeight`, &isFullScreen); err != nil {
					return errors.Wrap(err, "failed to execute JS expression")
				}
				if isFullScreen != param.wantFullscreenEnabled {
					return errors.Errorf("unexpected full screen state: got %v, want %v", isFullScreen, param.wantFullscreenEnabled)
				}
				return nil
			}, &testing.PollOptions{
				Timeout:  5 * time.Second,
				Interval: 1 * time.Second,
			}); err != nil {
				s.Error("Polling for having full screen failed: ", err)
			}
		})
	}
}
