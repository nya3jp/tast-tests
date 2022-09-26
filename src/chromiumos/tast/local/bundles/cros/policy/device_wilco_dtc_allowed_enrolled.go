// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceWilcoDtcAllowedEnrolled,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting the DeviceWilcoDtcAllowed policy by checking if the Wilco DTC Support Daemon is running",
		Contacts: []string{
			"lamzin@google.com", // Test author
			"vsavu@google.com",  // Test author
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "vm_host", "wilco"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DeviceWilcoDtcAllowed{}, pci.VerifiedFunctionalityOS),
		},
	})
}

// DeviceWilcoDtcAllowedEnrolled tests the DeviceWilcoDtcAllowed policy.
// TODO(b/189457904): rename to policy.DeviceWilcoDtcAllowed once stable and remote policy.DeviceWilcoDtcAllowed removed.
func DeviceWilcoDtcAllowedEnrolled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, tc := range []struct {
		name           string                       // name is the subtest name.
		value          policy.DeviceWilcoDtcAllowed // value is the policy value.
		affiliationIDs []string
	}{
		// User is affiliated in the next test cases.
		{
			name:           "affiliated_unset",
			value:          policy.DeviceWilcoDtcAllowed{Stat: policy.StatusUnset},
			affiliationIDs: []string{"default_affiliation_id"},
		},
		{
			name:           "affiliated_true",
			value:          policy.DeviceWilcoDtcAllowed{Val: true},
			affiliationIDs: []string{"default_affiliation_id"},
		},
		{
			name:           "affiliated_false",
			value:          policy.DeviceWilcoDtcAllowed{Val: false},
			affiliationIDs: []string{"default_affiliation_id"},
		},
		// User is not affiliated in the next test cases.
		{
			name:  "not_affiliated_unset",
			value: policy.DeviceWilcoDtcAllowed{Stat: policy.StatusUnset},
		},
		{
			name:  "not_affiliated_true",
			value: policy.DeviceWilcoDtcAllowed{Val: true},
		},
		{
			name:  "not_affiliated_false",
			value: policy.DeviceWilcoDtcAllowed{Val: false},
		},
	} {
		// DeviceWilcoDtcAllowed policy behaviour depends on:
		//   1) IsUserAffiliated flag;
		//   2) DeviceWilcoDtcAllowed policy value.
		//
		// There are 2 bugs related to the policy behaviour: crbug/1214031 and crbug/1215173.
		// So we have to be sure that IsUserAffiliated flag is updated and only
		// after it change policy value.
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			pb := policy.NewBlob()
			pb.DeviceAffiliationIds = tc.affiliationIDs
			pb.UserAffiliationIds = tc.affiliationIDs

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// After this point, IsUserAffiliated flag should be updated.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			// We should add policy value in the middle of 2 ServeBlobAndRefresh calls to be sure
			// that IsUserAffiliated flag is updated and policy handler is triggered.
			pb.AddPolicies([]policy.Policy{&tc.value})

			// After this point, the policy handler should be triggered.
			if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
				s.Fatal("Failed to serve and refresh: ", err)
			}

			// Only when allowed by policy and for affiliated users.
			wantRunning := tc.value.Val == true && len(tc.affiliationIDs) > 0

			supportdPID, err := wilco.SupportdPID(ctx)
			if err != nil {
				s.Fatal("Failed to get wilco_dtc_supportd process ID: ", err)
			}
			supportdRunning := supportdPID != 0
			if supportdRunning != wantRunning {
				s.Errorf("Unexpected wilco_dtc_supportd running status: got %t; want %t", supportdRunning, wantRunning)
			}

			vmPID, err := wilco.VMPID(ctx)
			if err != nil {
				s.Fatal("Failed to get wilco_dtc VM process ID: ", err)
			}
			vmRunning := vmPID != 0
			if vmRunning != wantRunning {
				s.Errorf("Unexpected wilco_dtc VM running status: got %t; want %t", vmRunning, wantRunning)
			}
		})
	}
}
