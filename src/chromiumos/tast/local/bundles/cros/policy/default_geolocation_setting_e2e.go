// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultGeolocationSettingE2E,
		Desc: "Behavior of DefaultGeolocationSetting policy, checking the location site settings after setting the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:policy_end2end"},
		Data:         []string{"default_geolocation_setting_index.html"},
		Timeout:      5 * time.Minute,
	})
}

// DefaultGeolocationSettingE2E tests the DefaultGeolocationSetting policy.
func DefaultGeolocationSettingE2E(ctx context.Context, s *testing.State) {

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	const tapeURL = "https://tape-307412.ey.r.appspot.com/setPolicy"

	const jsonStr = `{
		"requests":{
			"policyTargetKey":{
				"targetResource":"orgunits/03ph8a2z0rz2392"
			},
			"policyValue":{
				"policySchema":"chrome.users.Geolocation",
				"value":{
					"defaultGeolocationSetting":%d
				}
			},
			"updateMask":{
				"paths":"defaultGeolocationSetting"
			}
		}
	}`

	for _, param := range []struct {
		name            string
		nodeName        string                            // nodeName is the name of the toggle button node we want to check.
		wantAsk         bool                              // wantAsk states whether a dialog to ask for permission should appear or not.
		wantRestriction ui.RestrictionState               // wantRestriction is the wanted restriction state of the toggle button in the location settings.
		wantChecked     ui.CheckedState                   // wantChecked is the wanted checked state of the toggle button in the location settings.
		value           int                               // value is the value of the policy.
		policy          *policy.DefaultGeolocationSetting // value is the value of the policy.
	}{
		{
			name:            "allow",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         false,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateTrue,
			value:           1,
			policy:          &policy.DefaultGeolocationSetting{Val: 1},
		},
		{
			name:            "deny",
			nodeName:        "Blocked",
			wantAsk:         false,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateFalse,
			value:           2,
			policy:          &policy.DefaultGeolocationSetting{Val: 2},
		},
		{
			name:            "ask",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateTrue,
			value:           3,
			policy:          &policy.DefaultGeolocationSetting{Val: 3},
		},
		{
			name:            "unset",
			nodeName:        "Ask before accessing (recommended)",
			wantAsk:         true,
			wantRestriction: ui.RestrictionNone,
			wantChecked:     ui.CheckedStateTrue,
			value:           4,
			policy:          &policy.DefaultGeolocationSetting{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Ensure login screen.
			if err := upstart.RestartJob(ctx, "ui"); err != nil {
				s.Fatal("Failed to restart ui: ", err)
			}

			_, err := http.Post(tapeURL, "application/json", bytes.NewBuffer([]byte(fmt.Sprintf(jsonStr, param.value))))
			if err != nil {
				s.Fatal("Failed to set policies with TAPE: ", err)
			}

			cr, tconn, err := login(
				ctx,
				"tast-test1@tast-test.deviceadmin.goog",
				"dKFp3t7l!G", // Temporary password. Needs to be refreshed for each run.
			)
			if err != nil {
				s.Fatal("Failed to complete login: ", err)
			}

			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Ensure the policy is set.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Verify the policy was set
				return policyutil.Verify(ctx, tconn, []policy.Policy{param.policy})
			}, nil); err != nil {
				s.Error("Policy wasn't verified: ", err)
			}

			// Open the website.
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

				// TODO(crbug.com/1197511): investigate why this is needed.
				// Wait for a second before clicking the allow button as the click
				// won't be registered otherwise.
				testing.Sleep(ctx, time.Second)

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
		})
	}
}

func login(
	ctx context.Context,
	username,
	password string,
) (*chrome.Chrome, *chrome.TestConn, error) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start Chrome")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating test API connection failed")
	}

	return cr, tconn, nil
}
