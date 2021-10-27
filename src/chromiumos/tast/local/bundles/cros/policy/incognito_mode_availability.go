// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IncognitoModeAvailability,
		Desc: "Behavior of IncognitoModeAvailability policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               lacros.ChromeTypeLacros,
		}},
	})
}

func IncognitoModeAvailability(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// IncognitoModeAvailability policy values
	const (
		IncognitoModeEnabled  = 0
		IncognitoModeDisabled = 1
		IncognitoModeForced   = 2
	)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.IncognitoModeAvailability
	}{
		{
			name:  "unset",
			value: &policy.IncognitoModeAvailability{Stat: policy.StatusUnset},
		},
		//{
		//	name:  "enabled",
		//	value: &policy.IncognitoModeAvailability{Val: IncognitoModeEnabled},
		//},
		//{
		//	name:  "disabled",
		//	value: &policy.IncognitoModeAvailability{Val: IncognitoModeDisabled},
		//},
		//{
		//	name:  "forced",
		//	value: &policy.IncognitoModeAvailability{Val: IncognitoModeForced},
		//},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			_, l, _, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(lacros.ChromeType))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer lacros.CloseLacrosChrome(cleanupCtx, l)

			s.Log("ready")
			testing.Sleep(ctx, 30*time.Second)

			incognitoEnabled := param.value.Val != IncognitoModeDisabled

			// Open an incognito window
			if err := apps.LaunchChromeByShortcut(tconn, true)(ctx); err != nil {
				if incognitoEnabled {
					s.Fatal("Failed to open incognito browser window: ", err)
				}
			} else if !incognitoEnabled {
				s.Error("Expected accelerator not to be registered")
			}

			// Open a normal window
			if err := apps.LaunchChromeByShortcut(tconn, false)(ctx); err != nil {
				s.Fatal("Failed to open browser window: ", err)
			}

			windows, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get windows: ", err)
			}

			expectedWindows := 1
			if incognitoEnabled {
				expectedWindows = 2
			}
			if len(windows) != expectedWindows {
				s.Errorf("Unexpected number of open windows: got %d, want %d", len(windows), expectedWindows)
			}

			// TODO(crbug.com/1043875): check if windows are created as expected
		})
	}
}
