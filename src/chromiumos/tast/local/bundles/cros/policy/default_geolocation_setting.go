// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultGeolocationSetting,
		Desc: "Behavior of DefaultGeolocationSetting policy, checking the location site settings after setting the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"default_geolocation_setting_index.html"},
	})
}

// DefaultGeolocationSetting tests the DefaultGeolocationSetting policy.
func DefaultGeolocationSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

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
		wantChecked     checked.Checked                   // wantChecked is the wanted checked state of the toggle button in the location settings.
		wantRestriction restriction.Restriction           // wantRestriction is the wanted restriction state of the toggle button in the location settings.
		value           *policy.DefaultGeolocationSetting // value is the value of the policy.
	}{
		{
			name:            "unset",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantChecked:     checked.True,
			wantRestriction: restriction.None,
			value:           &policy.DefaultGeolocationSetting{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         false,
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
			value:           &policy.DefaultGeolocationSetting{Val: 1},
		},
		{
			name:            "deny",
			nodeName:        "Blocked",
			wantAsk:         false,
			wantChecked:     checked.False,
			wantRestriction: restriction.Disabled,
			value:           &policy.DefaultGeolocationSetting{Val: 2},
		},
		{
			name:            "ask",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
			value:           &policy.DefaultGeolocationSetting{Val: 3},
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

			// Open a website.
			conn, err := cr.NewConn(ctx, server.URL+"/default_geolocation_setting_index.html")
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

				if err := ui.LeftClickUntil(allowButton, ui.Gone(allowButton))(ctx); err != nil {
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
			if ec == 1 && param.wantChecked == checked.True {
				s.Error("Failed to get geolocation")
			} else if ec != 1 && param.wantChecked == checked.False {
				s.Error("Getting geolocation wasn't blocked")
			}

			if err := policyutil.SettingsPage(ctx, cr, "content/location").
				SelectNode(ctx, nodewith.
					Name(param.nodeName).
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
