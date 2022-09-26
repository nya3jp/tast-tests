// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type vkTestCase struct {
	name          string          // name is the subtest name.
	browserType   browser.Type    // browser type used in the subtest.
	wantedAllowVK bool            // wantedAllowVK describes if virtual keyboard is expected to be shown or not.
	policies      []policy.Policy // policies is the policies values.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboard,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of VirtualKeyboardEnabled and TouchVirtualKeyboardEnabled policies and their mixing by checking that the virtual keyboard (is/is not) displayed as requested by the policy",
		Contacts: []string{
			"kamilszarek@google.com",    // Test author of the merge.
			"giovax@google.com",         // Test author of the initial test for policy.VirtualKeyboardEnabled.
			"alexanderhartl@google.com", // Test author of the initial test for policy.TouchVirtualKeyboardEnabled.
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:    "both",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []vkTestCase{
					{
						name:          "vke_enabled-tvke_enabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_enabled-tvke_disabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_disabled-tvke_enabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_disabled-tvke_disabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_unset-tvke_enabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_enabled-tvke_unset",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
					{
						name:          "vke_unset-tvke_disabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_disabled-tvke_unset",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
					{
						name:          "vke_unset-tvke_unset",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}, &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:    "virtual",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []vkTestCase{
					{
						name:          "enabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "disabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "unset",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:    "touch_virtual",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []vkTestCase{
					{
						name:          "enabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "disabled",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "unset",
						browserType:   browser.TypeAsh,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:              "lacros_both",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []vkTestCase{
					{
						name:          "vke_enabled-tvke_enabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_enabled-tvke_disabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_disabled-tvke_enabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_disabled-tvke_disabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_unset-tvke_enabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}, &policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "vke_enabled-tvke_unset",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}, &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
					{
						name:          "vke_unset-tvke_disabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}, &policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "vke_disabled-tvke_unset",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}, &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
					{
						name:          "vke_unset-tvke_unset",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}, &policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:              "lacros_virtual",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []vkTestCase{
					{
						name:          "enabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "disabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "unset",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.VirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:              "lacros_touch_virtual",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []vkTestCase{
					{
						name:          "enabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: true,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: true}},
					},
					{
						name:          "disabled",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Val: false}},
					},
					{
						name:          "unset",
						browserType:   browser.TypeLacros,
						wantedAllowVK: false,
						policies:      []policy.Policy{&policy.TouchVirtualKeyboardEnabled{Stat: policy.StatusUnset}},
					},
				},
			},
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.VirtualKeyboardEnabled{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.TouchVirtualKeyboardEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// VirtualKeyboard applies VK related policies and uses browser's address bar
// to bring the virtual keyboard up. Then asserts according to expectations.
func VirtualKeyboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	uia := uiauto.New(tconn)

	tcs := s.Param().([]vkTestCase)

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Get tablet mode state.
			tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get tablet mode state: ", err)
			}

			// Restore the tablet mode to the initial state.
			defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

			// Set tablet mode to false - turn DUT to desktop mode.
			if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
				s.Fatal("Failed to set tablet mode enabled to false: ", err)
			}

			// Reset Chrome.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1254152): Modify browser setup after creating the new browser package.
			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, tc.browserType)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(
				ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.name)

			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Click the address bar.
			if err := uia.LeftClick(nodewith.ClassName("OmniboxViewViews").Role(role.TextField))(ctx); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}
			vkNode := nodewith.Name("Chrome OS Virtual Keyboard").Role(role.Keyboard)
			if tc.wantedAllowVK {
				// Confirm that the virtual keyboard exists.
				if err := uia.WaitUntilExists(vkNode)(ctx); err != nil {
					s.Errorf("Virtual keyboard did not show up: %s", err)
				}
			} else {
				// Confirm that the virtual keyboard does not exist.
				if err := uia.EnsureGoneFor(vkNode, 15*time.Second)(ctx); err != nil {
					s.Error("Virtual keyboard has shown")
				}
			}
		})
	}
}
