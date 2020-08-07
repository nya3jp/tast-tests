// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/policyutil"
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

	for _, param := range []struct {
		name           string
		nodeName       string                            // the name of the toggle button node we want to check.
		code           string                            // javascript code to be executed to get the allow dialog or the current position.
		wantAsk        bool                              // wantAsk states whether a dialog to ask for permission should appear or not.
		wantRestricted bool                              // wantRestricted is the wanted restriction state of the toggle button in the location settings.
		wantChecked    ui.CheckedState                   // wantChecked is the wanted checked state of the toggle button in the location settings.
		value          *policy.DefaultGeolocationSetting // value is the value of the policy.
	}{
		{
			name:           "unset",
			nodeName:       "Ask before accessing (recommended)",
			code:           "navigator.geolocation.getCurrentPosition(function(){}); 0;",
			wantAsk:        true,
			wantRestricted: false,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.DefaultGeolocationSetting{Stat: policy.StatusUnset},
		},
		{
			name:           "allow",
			nodeName:       "Ask before accessing (recommended)",
			code:           "new Promise(function(resolve, reject) {navigator.geolocation.getCurrentPosition(resolve, reject, {timeout: 5000});}).then((position) => {return 0;}).catch((err) => {return err.code;});",
			wantAsk:        false,
			wantRestricted: true,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.DefaultGeolocationSetting{Val: 1},
		},
		{
			name:           "deny",
			nodeName:       "Blocked",
			code:           "new Promise(function(resolve, reject) {navigator.geolocation.getCurrentPosition(resolve, reject, {timeout: 5000});}).then((position) => {return 0;}).catch((err) => {return err.code;});",
			wantAsk:        false,
			wantRestricted: true,
			wantChecked:    ui.CheckedStateFalse,
			value:          &policy.DefaultGeolocationSetting{Val: 2},
		},
		{
			name:           "ask",
			nodeName:       "Ask before accessing (recommended)",
			code:           "navigator.geolocation.getCurrentPosition(function(){}); 0;",
			wantAsk:        true,
			wantRestricted: true,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.DefaultGeolocationSetting{Val: 3},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API to use it with the ui library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Open a website.
			conn, err := cr.NewConn(ctx, server.URL+"/default_geolocation_setting_index.html")
			if err != nil {
				s.Fatal("Creating renderer failed: ", err)
			}
			defer conn.Close()

			// Try to access the geolocation or trigger the allow dialog.
			var ec int
			if err := conn.Eval(ctx, param.code, &ec); err != nil {
				s.Fatal("Failed to request current position: ", err)
			}

			// Check if we got an error while requesting the current position.
			if ec != 0 && param.wantChecked == ui.CheckedStateTrue {
				s.Error("Failed to get geolocation")
			}

			// Check whether an allow dialog to ask for permission opens.
			if _, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeWindow,
				Name: fmt.Sprintf("%s wants to", server.URL),
			}, 15*time.Second); err != nil && !errors.Is(err, ui.ErrNodeDoesNotExist) {
				s.Fatal("Finding allow dialog to node failed: ", err)
			} else {
				if d := !errors.Is(err, ui.ErrNodeDoesNotExist); param.wantAsk != d {
					s.Errorf("Unexpected existence of dialog to ask for permission: got %t; want %t", d, param.wantAsk)
				}
			}

			// Open settings page where the affected toggle button can be found.
			sconn, err := cr.NewConn(ctx, "chrome://settings/content/location")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer sconn.Close()

			// Find the toggle button node.
			node, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeToggleButton,
				Name: param.nodeName,
			}, 15*time.Second)
			if err != nil {
				s.Fatalf("Finding %s node failed: %v", param.nodeName, err)
			}
			defer node.Release(ctx)

			// Check the restriction setting of the toggle button.
			if restricted := (node.Restriction == ui.RestrictionDisabled || node.Restriction == ui.RestrictionReadOnly); restricted != param.wantRestricted {
				s.Logf("The restriction attribute is %q", node.Restriction)
				s.Errorf("Unexpected toggle button restriction in the settings: got %t; want %t", restricted, param.wantRestricted)
			}

			if node.Checked != param.wantChecked {
				s.Errorf("Unexpected toggle button checked state in the settings: got %t; want %t", node.Checked, param.wantChecked)
			}

		})
	}
}
