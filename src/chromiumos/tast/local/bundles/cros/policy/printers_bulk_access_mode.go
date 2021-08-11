// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/kylelemons/godebug/pretty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrintersBulkAccessMode,
		Desc: "Verify behavior of PrintersBulkAccessMode user policy",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// sameIDs compares expected and actual IDs as sets, that is, checks if they contain the same
// values ignoring the order. It returns a human readable diff between the values, which is an empty
// string if the values are the same.
func sameIDs(want, got []string) string {
	gotMap := make(map[string]bool)
	for _, g := range got {
		gotMap[g] = true
	}

	wantMap := make(map[string]bool)
	for _, w := range want {
		wantMap[w] = true
	}

	return pretty.Compare(wantMap, gotMap)
}

func PrintersBulkAccessMode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// All the common policies that define the printers configuration, allowlist and blocklist.
	commonPolicies := []policy.Policy{
		&policy.PrintersBulkAllowlist{Val: []string{"both", "wl"}},
		&policy.PrintersBulkBlocklist{Val: []string{"both", "bl"}},
		&policy.PrintersBulkConfiguration{Val: &policy.PrintersBulkConfigurationValue{
			Url:  "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/printers.json",
			Hash: "7a052c5e4f23c159668148df2a3c202bed4d65749cab5ecd0fa7db211c12a3b8",
		}},
	}

	for _, param := range []struct {
		name        string
		expectedIDs []string // expectedIDs is the expected list of ids for each configuration.
		policies    []policy.Policy
	}{
		{
			name:        "all_except_blocklist",
			expectedIDs: []string{"wl", "other"},
			policies: append(
				commonPolicies,
				&policy.PrintersBulkAccessMode{Val: 0},
			),
		},
		{
			name:        "allowlist",
			expectedIDs: []string{"wl", "both"},
			policies: append(
				commonPolicies,
				&policy.PrintersBulkAccessMode{Val: 1},
			),
		},
		{
			name:        "all",
			expectedIDs: []string{"bl", "wl", "other", "both"},
			policies: append(
				commonPolicies,
				&policy.PrintersBulkAccessMode{Val: 2},
			),
		},
		{
			name:        "unset",
			expectedIDs: []string{"bl", "wl", "other", "both"},
			policies: append(
				commonPolicies,
				&policy.PrintersBulkAccessMode{Stat: policy.StatusUnset},
			),
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Retrieve Printers seen by user.
			printers := make([]map[string]string, 0)
			if err := tconn.Call(ctx, &printers, `tast.promisify(chrome.autotestPrivate.getPrinterList)`); err != nil {
				s.Fatal("Failed to evaluate JS expression and get printers: ", err)
			}

			// Get Printers IDs.
			foundIDs := make(map[string]bool)
			ids := make([]string, 0)
			for _, printer := range printers {
				if id, ok := printer["printerId"]; ok {
					foundIDs[id] = true
					ids = append(ids, id)
				} else {
					s.Fatal("Missing printerId field")
				}
			}
			if len(foundIDs) < len(printers) {
				s.Fatal("Received response contains duplicates")
			}

			if diff := sameIDs(param.expectedIDs, ids); diff != "" {
				s.Error(errors.Errorf("unexpected IDs (-want +got): %v", diff))
			}
		})
	}
}
