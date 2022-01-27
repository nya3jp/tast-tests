// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceAllowBluetooth,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Behavior of DeviceAllowBluetooth policy",
		Contacts: []string{
			"jeroendh@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      fixture.ChromePolicyLoggedIn,
	})
}

func DeviceAllowBluetooth(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.DeviceAllowBluetooth
		// if the bluetooth button is expected to be available in the system tray.
		expectedPodAvailable bool
	}{
		{
			name:                 "true",
			value:                &policy.DeviceAllowBluetooth{Val: true},
			expectedPodAvailable: true,
		},
		{
			name:                 "false",
			value:                &policy.DeviceAllowBluetooth{Val: false},
			expectedPodAvailable: false,
		},
		{
			name:                 "unset",
			value:                &policy.DeviceAllowBluetooth{Stat: policy.StatusUnset},
			expectedPodAvailable: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			found, err := CheckTrayForBluetoothPod(tconn, ctx)
			if err != nil {
				s.Fatal("Failed to check for the bluetooth button in the system tray: ", err)
			}

			if found != param.expectedPodAvailable {
				if param.expectedPodAvailable {
					s.Fatal("Bluetooth option is not available and it should be.")
				} else {
					s.Fatal("Bluetooth option is available and it should not be.")
				}
			}
		})
	}
}

// To check if the policy is correctly applied, we search for the bluetooth
// button in the system tray.
//   - When the policy is true/unset the bluetooth button should be available.
//   - When the policy is false the bluetooth button should *not* be available.
func CheckTrayForBluetoothPod(tconn *chrome.TestConn, ctx context.Context) (bool, error) {
	ui := uiauto.New(tconn)

	// Open system tray.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "Failed to open the system tray")
	}
	defer quicksettings.Hide(ctx, tconn)

	// Find the Bluetooth button.
	found, err := ui.IsNodeFound(ctx, quicksettings.PodIconButton(quicksettings.SettingPodBluetooth))
	if err != nil {
		return false, errors.Wrap(err, "Failure searching for bluetooth button")
	}

	return found, nil
}
