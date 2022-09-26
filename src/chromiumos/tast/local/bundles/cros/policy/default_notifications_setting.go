// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultNotificationsSetting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DefaultNotificationsSetting policy, checks the notification permission in JavaScript at different policy values",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultNotificationsSetting{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// DefaultNotificationsSetting tests the DefaultNotificationsSetting policy.
func DefaultNotificationsSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// radioButtonNames is a list of UI element names in the notification settings page.
	// The order of the strings should follow the order in the settings page.
	// wantRestriction and wantChecked entries are expected to follow this order as well.
	radioButtonNames := []string{
		"Sites can ask to send notifications",
		"Use quieter messaging",
		"Don't allow sites to send notifications",
	}

	for _, param := range []struct {
		name            string
		wantPermission  string                    // the expected answer for the JS query
		wantRestriction []restriction.Restriction // the expected restriction states of the radio buttons in radioButtonNames
		wantChecked     []checked.Checked         // the expected checked states of the radio buttons in radioButtonNames
		value           *policy.DefaultNotificationsSetting
	}{
		{
			name:            "unset",
			wantPermission:  "default",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None, restriction.None},
			wantChecked:     []checked.Checked{checked.True, checked.False, checked.False},
			value:           &policy.DefaultNotificationsSetting{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantPermission:  "granted",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False, checked.False},
			value:           &policy.DefaultNotificationsSetting{Val: 1}, // Allow sites to show desktop notifications.
		},
		{
			name:            "deny",
			wantPermission:  "denied",
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.Disabled, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.False, checked.False, checked.True},
			value:           &policy.DefaultNotificationsSetting{Val: 2}, // Do not allow any site to show desktop notifications.
		},
		{
			name:            "ask",
			wantPermission:  "default",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False, checked.False},
			value:           &policy.DefaultNotificationsSetting{Val: 3}, // Ask every time a site wants to show desktop notifications.
		},
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

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open notification settings.
			conn, err := br.NewConn(ctx, "chrome://settings/content/notifications")
			if err != nil {
				s.Fatal("Failed to open notification settings: ", err)
			}
			defer conn.Close()

			var permission string
			if err := conn.Eval(ctx, "Notification.permission", &permission); err != nil {
				s.Fatal("Failed to eval: ", err)
			} else if permission != param.wantPermission {
				s.Errorf("Unexpected permission value; got %s, want %s", permission, param.wantPermission)
			}

			// Check the state of the buttons.
			for i, radioButtonName := range radioButtonNames {
				if err := policyutil.CurrentPage(cr).
					SelectNode(ctx, nodewith.
						Role(role.RadioButton).
						Name(radioButtonName)).
					Restriction(param.wantRestriction[i]).
					Checked(param.wantChecked[i]).
					Verify(); err != nil {
					s.Errorf("Unexpected settings state for the %q button: %v", radioButtonName, err)
				}
			}
		})
	}
}
