// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
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
		Func:         DefaultGeolocationSetting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DefaultGeolocationSetting policy, checking the location site settings after setting the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{"default_geolocation_setting_index.html"},
		SearchFlags: []*testing.StringPair{{
			Key:   "feature_id",
			Value: "screenplay-763459eb-41b2-4e31-9381-91808acb7c97",
		}, {
			Key:   "feature_id",
			Value: "screenplay-9279088e-0489-4fac-bc4d-af79c9f4038f",
		},
			pci.SearchFlag(&policy.DefaultGeolocationSetting{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// DefaultGeolocationSetting tests the DefaultGeolocationSetting policy.
func DefaultGeolocationSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// radioButtonNames is a list of UI element names in the location settings page.
	// The order of the strings should follow the order in the settings page.
	// wantRestriction and wantChecked entries are expected to follow this order as well.
	radioButtonNames := []string{
		"Sites can ask for your location",
		"Don't allow sites to see your location",
	}

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn)

	for _, param := range []struct {
		name            string
		nodeName        string                            // nodeName is the name of the toggle button node we want to check.
		wantAsk         bool                              // wantAsk states whether a dialog to ask for permission should appear or not.
		wantRestriction []restriction.Restriction         // the expected restriction states of the radio buttons in radioButtonNames
		wantChecked     []checked.Checked                 // the expected checked states of the radio buttons in radioButtonNames
		value           *policy.DefaultGeolocationSetting // value is the value of the policy.
	}{
		{
			name:            "unset",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None},
			wantChecked:     []checked.Checked{checked.True, checked.False},
			value:           &policy.DefaultGeolocationSetting{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         false,
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False},
			value:           &policy.DefaultGeolocationSetting{Val: 1},
		},
		{
			name:            "deny",
			nodeName:        "Blocked",
			wantAsk:         false,
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.False, checked.True},
			value:           &policy.DefaultGeolocationSetting{Val: 2},
		},
		{
			name:            "ask",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False},
			value:           &policy.DefaultGeolocationSetting{Val: 3},
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

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open a website.
			conn, err := br.NewConn(ctx, server.URL+"/default_geolocation_setting_index.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			// Start a go routine before requesting the current position as the Eval()
			// function will block when a dialog to ask for permission appears.
			// The routine will then click the allow button in the dialog.
			ch := make(chan error, 1)
			go func() {
				allowButton := nodewith.Name("Allow").Role(role.Button)

				if err = ui.WaitUntilExists(allowButton)(ctx); err != nil {
					if param.wantAsk {
						s.Error("Allow button not found: ", err)
					}
					ch <- nil
					return
				}

				if !param.wantAsk {
					s.Error("Unexpected dialog to ask for geolocation access permission found")
				}

				// TODO(crbug.com/1197511): investigate why this is needed.
				// Wait for a second before clicking the allow button as the click
				// won't be registered otherwise.
				testing.Sleep(ctx, time.Second)

				if err := ui.DoDefaultUntil(allowButton, ui.Gone(allowButton))(ctx); err != nil {
					s.Fatal("Failed to click the Allow button: ", err)
				}

				ch <- nil
			}()

			// Try to access the geolocation.
			var ec int // ec is used to store the error_code.
			if err := conn.Eval(ctx, "requestPosition()", &ec); err != nil {
				s.Fatal("Failed to request current position: ", err)
			}

			if err := <-ch; err != nil {
				s.Error("Failed to execute the routine to click the allow button: ", err)
			}

			// Check if we got an error while requesting the current position.
			if ec == 1 && param.wantChecked[0] == checked.True {
				s.Error("Failed to get geolocation")
			} else if ec != 1 && param.wantChecked[0] == checked.False {
				s.Error("Getting geolocation wasn't blocked")
			}

			// Check the state of the buttons.
			for i, radioButtonName := range radioButtonNames {
				if err := policyutil.SettingsPage(ctx, cr, br, "content/location").
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
