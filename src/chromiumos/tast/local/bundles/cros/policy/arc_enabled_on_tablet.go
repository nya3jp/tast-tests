// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcEnabledOnTablet,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of ArcEnabled policy on tablet form factor, checking whether ARC is enabled after setting the policy",
		Contacts: []string{
			"yaohuali@google.com", // Test author
			"arc-commercial@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "tablet_form_factor"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "tablet_form_factor"},
			ExtraAttr:         []string{"informational"},
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ArcEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// ArcEnabledOnTablet tests the ArcEnabled policy on tablet form factor.
// On tablet only, when ARC is disabled by policy, Play Store icon still appears on shelf.
func ArcEnabledOnTablet(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(policy.NewBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.ExtraArgs("--arc-availability=officially-supported"),
		chrome.DeferLogin(),
	)
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

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
			value:       &policy.ArcEnabled{Val: true, Stat: policy.StatusSet},
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
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Look for the Play Store icon and click.
			// Polling till the icon is found or the timeout is reached.
			uia := uiauto.New(tconn)
			notFoundError := errors.New("Play Store icon is not found yet")
			err := testing.Poll(ctx, func(ctx context.Context) error {
				if found, err := uia.IsNodeFound(ctx, nodewith.Name(apps.PlayStore.Name).ClassName("ash/ShelfAppButton")); err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						return err
					}
					return testing.PollBreak(errors.Wrap(err, "failed to check Play Store icon"))
				} else if found {
					return nil
				}
				return notFoundError
			}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second})

			// We expect Play icon to always appear on tablet, regardless whether ARC is enabled by policy.
			if err != nil {
				s.Fatal("Failed to confirm the Play Store icon: ", err)
			}

			// Click on Play Store icon and see what pops out.
			if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
				s.Fatal("Failed to launch Play Store: ", err)
			}

			if param.wantEnabled {
				// ARC opt-in is expected to happen (but fail, due to the fake policy we gave).
				arcOptInUI := nodewith.Name("Google Play apps and services").Role(role.StaticText)
				if err := uia.WithTimeout(50 * time.Second).WaitUntilExists(arcOptInUI)(ctx); err != nil {
					s.Fatal("Failed to see ARC Opt-In UI: ", err)
				}
				// Reset Chrome will close the ARC opt-in window.
			} else {
				// On tablet, a pop-up window will inform user that Play Store is not available.
				popupUI := nodewith.Name("This app requires access to the Play Store").Role(role.TitleBar)
				if err := uia.WithTimeout(3 * time.Second).WaitUntilExists(popupUI)(ctx); err != nil {
					s.Fatal("Failed to see pop-up window: ", err)
				}

				// If we click the OK button immediately when the pop-up appears, the pop-up don't be dismissed.
				testing.Sleep(ctx, 1*time.Second)

				// Click OK button to dismiss pop-up window.
				button := nodewith.Name("OK").Role(role.Button)
				if e := uia.WithTimeout(3 * time.Second).LeftClick(button)(ctx); e != nil {
					s.Fatal("Failed to dismiss pop-up window")
				}

				// Ensure pop-up window is gone. Otherwise it would interfere with next iteration.
				if err := uia.WithTimeout(2 * time.Second).WaitUntilGone(button)(ctx); err != nil {
					s.Fatal("Failed to wait for pop-up window to be dismissed")
				}
			}
		})
	}
}
