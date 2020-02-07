// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: IncognitoModeAvailability,
		Desc: "Behavior of IncognitoModeAvailability policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func IncognitoModeAvailability(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// IncognitoModeAvailability policy values
	const (
		IncognitoModeEnabled  = 0
		IncognitoModeDisabled = 1
		IncognitoModeForced   = 2
	)

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.IncognitoModeAvailability
	}{
		{
			name:  "unset",
			value: &policy.IncognitoModeAvailability{Stat: policy.StatusUnset},
		},
		{
			name:  "enabled",
			value: &policy.IncognitoModeAvailability{Val: IncognitoModeEnabled},
		},
		{
			name:  "disabled",
			value: &policy.IncognitoModeAvailability{Val: IncognitoModeDisabled},
		},
		{
			name:  "forced",
			value: &policy.IncognitoModeAvailability{Val: IncognitoModeForced},
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

			// Run actual test.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			const (
				newBrowser          string = "{keyCode: 'n', shift: false, control: true, alt: false, search: false, pressed: true}"
				newIncognitoBrowser string = "{keyCode: 'n', shift: true, control: true, alt: false, search: false, pressed: true}"
			)

			triggerAccelerator := func(accelerator string) error {
				expr := fmt.Sprintf(`
					(async () => {
						let accelerator = %s;
						await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accelerator);
						accelerator.pressed = false;
						await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accelerator)
					})()`, accelerator)

				if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
					return errors.Wrap(err, "could not open browser")
				}

				return nil
			}

			incognitoEnabled := param.value.Val != IncognitoModeDisabled

			// Open an incognito window
			if err := triggerAccelerator(newIncognitoBrowser); err != nil {
				if incognitoEnabled {
					s.Fatal("Failed to open incognito browser window: ", err)
				}
			} else if !incognitoEnabled {
				s.Error("Expected accelerator not to be registered")
			}

			// Open a normal window
			if err := triggerAccelerator(newBrowser); err != nil {
				s.Fatal("Failed to open browser window: ", err)
			}

			windows, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get windows: ", err)
			}

			expectedWindows := 1
			if incognitoEnabled {
				expectedWindows = 2
			}
			if len(windows) != expectedWindows {
				s.Errorf("Unexpected number of open windows: got %d, want %d", len(windows), expectedWindows)
			}

			// TODO(crbug.com/1043875): check if windows are created as expected
		})
	}
}
