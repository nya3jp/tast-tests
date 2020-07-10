// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AllowDeletingBrowserHistory,
		Desc: "Behavior of AllowDeletingBrowserHistory policy, checking the correspoding checkbox states (restriction and value) after setting the policy",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// AllowDeletingBrowserHistory tests the AllowDeletingBrowserHistory policy.
func AllowDeletingBrowserHistory(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	for _, param := range []struct {
		name           string
		wantRestricted bool                                // wantRestricted is the wanted state of the Browsing history checkbox.
		wantValue      bool                                // wantValue is the wanted value of the Browsing history checkbox.
		value          *policy.AllowDeletingBrowserHistory // value is the policy value.
	}{
		{
			name:           "unset",
			wantRestricted: false,
			wantValue:      true,
			value:          &policy.AllowDeletingBrowserHistory{Stat: policy.StatusUnset},
		},
		{
			name:           "allow",
			wantRestricted: false,
			wantValue:      true,
			value:          &policy.AllowDeletingBrowserHistory{Val: true},
		},
		{
			name:           "deny",
			wantRestricted: true,
			wantValue:      false,
			value:          &policy.AllowDeletingBrowserHistory{Val: false},
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

			// Run actual test.
			conn, err := cr.NewConn(ctx, "chrome://settings/clearBrowserData")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			settingValueAndRestriction := func(key string) (bool, bool) {
				checkbox := make(map[string]interface{})
				request := fmt.Sprintf(
					`new Promise(function(resolve, reject) {
						chrome.settingsPrivate.getAllPrefs(function(root) {
							if (chrome.runtime.lastError) {
								reject(new Error(chrome.runtime.lastError.message));
								return;
							}
							resolve(root.find(obj => {
								return obj.key == "%s"
							}))
						})
					})`, key)
				if err := conn.Eval(ctx, request, &checkbox); err != nil {
					s.Fatal("Failed to evaluate JS code: ", err)
				}

				value, ok := checkbox["value"].(bool)
				if !ok {
					s.Error("Unexpected value type in JS response")
				}

				// In case of no restriction, the enforcement key is not present.
				_, ok = checkbox["enforcement"]
				if !ok {
					return value == true, false
				}

				enforcement, ok := checkbox["enforcement"].(string)
				if !ok {
					s.Error("Unexpected enforcement type in JS response")
				}

				return value == true, enforcement == "ENFORCED"
			}

			for _, settingName := range []string{
				"browser.clear_data.browsing_history_basic",
				"browser.clear_data.browsing_history",
				"browser.clear_data.download_history",
			} {
				value, restricted := settingValueAndRestriction(settingName)

				if value != param.wantValue {
					s.Errorf("Unexpected %s checkbox value: got %t; want %t", settingName, value, param.wantValue)
				}

				if restricted != param.wantRestricted {
					s.Errorf("Unexpected %s checkbox restriction: got %t; want %t", settingName, restricted, param.wantRestricted)
				}
			}
		})
	}

}
