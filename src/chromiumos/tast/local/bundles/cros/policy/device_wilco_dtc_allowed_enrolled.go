// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceWilcoDtcAllowedEnrolled,
		Desc: "Test setting the DeviceWilcoDtcAllowed policy by checking if the Wilco DTC Support Daemon is running",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"vsavu@google.com",  // Test author
			"chromeos-wilco@google.com",
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "wilco"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

// DeviceWilcoDtcAllowedEnrolled tests the DeviceWilcoDtcAllowed policy.
// TODO(b/189457904): rename to policy.DeviceWilcoDtcAllowed once stable and remote policy.DeviceWilcoDtcAllowed removed.
func DeviceWilcoDtcAllowedEnrolled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, tc := range []struct {
		name        string                       // name is the subtest name.
		value       policy.DeviceWilcoDtcAllowed // value is the policy value.
		wantRunning bool
	}{
		{
			name:        "unset",
			value:       policy.DeviceWilcoDtcAllowed{Stat: policy.StatusUnset},
			wantRunning: false,
		},
		{
			name:        "true",
			value:       policy.DeviceWilcoDtcAllowed{Val: true},
			wantRunning: true,
		},
		{
			name:        "false",
			value:       policy.DeviceWilcoDtcAllowed{Val: false},
			wantRunning: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			pb := fakedms.NewPolicyBlob()
			pb.AddPolicies([]policy.Policy{&tc.value})
			pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
			pb.UserAffiliationIds = []string{"default_affiliation_id"}

			// Perform cleanup.
			if err := policyutil.ResetChromeWithBlob(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}
			// TODO(crbug/1214031): Remove this useless refresh after fixing bug with device affiliation IDs.
			if err := policyutil.RefreshChromePolicies(ctx, cr); err != nil {
				s.Fatal("Failed to refresh policies: ", err)
			}

			supportdPID, err := wilco.SupportdPID(ctx)
			if err != nil {
				s.Fatal("Failed to get wilco_dtc_supportd process ID: ", err)
			}
			supportdRunning := supportdPID != 0
			if supportdRunning != tc.wantRunning {
				s.Errorf("Unexpected wilco_dtc_supportd running status: got %t; want %t", supportdRunning, tc.wantRunning)
			}

			vmPID, err := wilco.VMPID(ctx)
			if err != nil {
				s.Fatal("Failed to get wilco_dtc VM process ID: ", err)
			}
			vmRunning := vmPID != 0
			if vmRunning != tc.wantRunning {
				s.Errorf("Unexpected wilco_dtc VM running status: got %t; want %t", vmRunning, tc.wantRunning)
			}
		})
	}
}
