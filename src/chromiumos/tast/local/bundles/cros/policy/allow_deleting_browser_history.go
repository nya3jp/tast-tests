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
		Desc: "Behavior of AllowDeletingBrowserHistory policy",
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
		name       string                              // name is the subtest name.
		restricted bool                                // restricted is the state of the Browsing history checkbox.
		setvalue   bool                                // setvalue is the value of the Browsing history checkbox.
		value      *policy.AllowDeletingBrowserHistory // value is the policy value.
	}{
		{
			name:       "Unset",
			restricted: false,
			setvalue:   true,
			value:      &policy.AllowDeletingBrowserHistory{Stat: policy.StatusUnset},
		},
		{
			name:       "Allow",
			restricted: false,
			setvalue:   true,
			value:      &policy.AllowDeletingBrowserHistory{Val: true},
		},
		{
			name:       "Deny",
			restricted: true,
			setvalue:   false,
			value:      &policy.AllowDeletingBrowserHistory{Val: false},
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
			settingValueAndRestriction := func(key string) (bool, bool) {
				conn, err := cr.NewConn(ctx, "chrome://settings/clearBrowserData")
				if err != nil {
					s.Fatal("Failed to connect to chrome: ", err)
				}
				defer conn.Close()

				checkbox := make(map[string]interface{})
				request := fmt.Sprintf(
					`new Promise(function(resolve, reject) {
						chrome.settingsPrivate.getAllPrefs(function(root) {
							resolve(root.find(obj => {
								return obj.key == "%s"
							}))
						})
					})`, key)
				if err := conn.Eval(ctx, request, &checkbox); err != nil {
					s.Fatal("Failed to get JS Promise: ", err)
				}
				return checkbox["value"] == true, checkbox["enforcement"] == "ENFORCED"
			}

			settingName := "browser.clear_data.browsing_history"
			value, restricted := settingValueAndRestriction(settingName)

			if value != param.setvalue {
				s.Errorf("%s value is %t instead of %t", settingName, value, param.setvalue)
			}

			if restricted != param.restricted {
				s.Errorf("%s restriction is %t instead of %t", settingName, restricted, param.restricted)
			}
		})
	}

}
