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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
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
		Pre:          pre.User,
		Data:         []string{"default_geolocation_setting_index.html"},
	})
}

// DefaultGeolocationSetting tests the DefaultGeolocationSetting policy.
func DefaultGeolocationSetting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		nodeName        string                            // nodeName is the name of the toggle button node we want to check.
		wantAsk         bool                              // wantAsk states whether a dialog to ask for permission should appear or not.
		wantRestriction ui.RestrictionState               // wantRestriction is the wanted restriction state of the toggle button in the location settings.
		wantChecked     ui.CheckedState                   // wantChecked is the wanted checked state of the toggle button in the location settings.
		value           *policy.DefaultGeolocationSetting // value is the value of the policy.
	}{
		{
			name:            "unset",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantRestriction: ui.RestrictionNone,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.DefaultGeolocationSetting{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         false,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.DefaultGeolocationSetting{Val: 1},
		},
		{
			name:            "deny",
			nodeName:        "Blocked",
			wantAsk:         false,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateFalse,
			value:           &policy.DefaultGeolocationSetting{Val: 2},
		},
		{
			name:            "ask",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.DefaultGeolocationSetting{Val: 3},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

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
				params := ui.FindParams{
					Role: ui.RoleTypeButton,
					Name: "Allow",
				}

				if err := policyutil.WaitUntilExistsStatus(ctx, tconn, params, param.wantAsk, 30*time.Second); err != nil {
					ch <- errors.Wrap(err, "failed to confirm the desired status of the allow button")
					return
				}

				// Return if there is no dialog.
				if !param.wantAsk {
					ch <- nil
					return
				}

				// Get the allow button.
				node, err := ui.Find(ctx, tconn, params)
				if err != nil {
					ch <- errors.Wrap(err, "failed to find allow button node")
					return
				}

				// This button takes a bit before it is clickable.
				// Keep clicking it until the click is received and the dialog closes.
				condition := func(ctx context.Context) (bool, error) {
					exists, err := ui.Exists(ctx, tconn, params)
					return !exists, err
				}
				opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
				if err := node.LeftClickUntil(ctx, condition, &opts); err != nil {
					ch <- errors.Wrap(err, "failed to click allow button")
					return
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
			if ec == 1 && param.wantChecked == ui.CheckedStateTrue {
				s.Error("Failed to get geolocation")
			} else if ec != 1 && param.wantChecked == ui.CheckedStateFalse {
				s.Error("Getting geolocation wasn't blocked")
			}

			// Open settings page where the affected toggle button can be found.
			if err := policyutil.VerifySettingsState(ctx, cr, "chrome://settings/content/location",
				ui.FindParams{
					Role: ui.RoleTypeToggleButton,
					Name: param.nodeName,
				},
				ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.wantRestriction,
						"checked":     param.wantChecked,
					},
				},
			); err != nil {
				s.Error("Unexpected settings state: ", err)
			}
		})
	}
}
