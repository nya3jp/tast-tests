// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ArcEnabled,
		Desc: "Behavior of ArcEnabled policy, checking whether ARC is enabled after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// ArcEnabled tests the ArcEnabled policy.
func ArcEnabled(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth(pre.Username, pre.Password, pre.GaiaID),
		chrome.DMSPolicy(fdms.URL),
		chrome.ExtraArgs("--arc-availability=officially-supported"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name        string
		wantEnabled bool               // wantEnabled is whether we want ARC enabled.
		value       *policy.ArcEnabled // value is the value of the policy.
	}{
		{
			name:        "enable",
			wantEnabled: true,
			value:       &policy.ArcEnabled{Val: true},
		},
		{
			name:        "disable",
			wantEnabled: false,
			value:       &policy.ArcEnabled{Val: false},
		},
		{
			name:        "unset",
			wantEnabled: false,
			value:       &policy.ArcEnabled{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open launcher if it is not open already.
			if exists, err := ui.Exists(ctx, tconn, ui.FindParams{ClassName: "AppListView"}); err != nil || !exists {
				// Open Launcher.
				if err := kb.Accel(ctx, "Search"); err != nil {
					s.Fatal("Failed to type Search: ", err)
				}

				if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "AppListView"}, 10*time.Second); err != nil {
					s.Fatal("Failed to open launcher: ", err)
				}
			}

			// Look for the Play Store icon.
			if err := policyutil.WaitUntilExistsStatus(ctx, tconn, ui.FindParams{
				Name:      apps.PlayStore.Name,
				ClassName: "SearchResultSuggestionChipView",
			}, param.wantEnabled, 15*time.Second); err != nil {
				s.Error("Could not confirm the desired status of the Play Store suggestion chip view: ", err)
			}
		})
	}
}
