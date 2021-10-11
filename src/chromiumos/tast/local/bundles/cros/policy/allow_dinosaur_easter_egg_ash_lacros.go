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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AllowDinosaurEasterEggAshLacros,
		Desc: "Behavior of AllowDinosaurEasterEgg policy on both Ash and Lacros browser",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"vsavu@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "ash",
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

func AllowDinosaurEasterEggAshLacros(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.AllowDinosaurEasterEgg
	}{
		{
			name:  "true",
			value: &policy.AllowDinosaurEasterEgg{Val: true},
		},
		{
			name:  "false",
			value: &policy.AllowDinosaurEasterEgg{Val: false},
		},
		{
			name:  "unset",
			value: &policy.AllowDinosaurEasterEgg{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			_, l, br, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(lacros.ChromeType))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer lacros.CloseLacrosChrome(cleanupCtx, l)

			// Run actual test.
			conn, err := br.NewConn(ctx, "chrome://dino")
			if err != nil {
				s.Fatal("Failed to connect to the browser: ", err)
			}
			defer conn.Close()

			var isBlocked bool
			if err := conn.Eval(ctx, `document.querySelector('* #main-frame-error div.snackbar') === null`, &isBlocked); err != nil {
				s.Fatal("Could not read from dino page: ", err)
			}

			expectedBlocked := param.value.Stat != policy.StatusUnset && param.value.Val

			if isBlocked != expectedBlocked {
				s.Errorf("Unexpected blocked behavior: got %t; want %t", isBlocked, expectedBlocked)
			}
		})
	}
}
