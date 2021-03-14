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
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrintersBulkAccessMode,
		Desc: "Verify behavior of PrintersBulkAccessMode user policy",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// sameIDs compares expected and actual ids as sets, that is, checks if they contain the same
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
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name        string
		expectedIDs []string // expectedIDs is the expcted list of ids for each configuration.
		policies    []policy.Policy
	}{
		{
			name:        "all_except_blocklist",
			expectedIDs: []string{"wl", "other"},
			policies: []policy.Policy{
				&policy.PrintersBulkAccessMode{Val: 0},
				&policy.PrintersBulkAllowlist{Val: []string{"both", "wl"}},
				&policy.PrintersBulkBlocklist{Val: []string{"both", "bl"}},
				&policy.PrintersBulkConfiguration{Val: &policy.PrintersBulkConfigurationValue{Url: "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/printers.json", Hash: "7a052c5e4f23c159668148df2a3c202bed4d65749cab5ecd0fa7db211c12a3b8"}},
			},
		},
		{
			name:        "allowlist",
			expectedIDs: []string{"wl", "both"},
			policies: []policy.Policy{
				&policy.PrintersBulkAccessMode{Val: 1},
				&policy.PrintersBulkAllowlist{Val: []string{"both", "wl"}},
				&policy.PrintersBulkBlocklist{Val: []string{"both", "bl"}},
				&policy.PrintersBulkConfiguration{Val: &policy.PrintersBulkConfigurationValue{Url: "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/printers.json", Hash: "7a052c5e4f23c159668148df2a3c202bed4d65749cab5ecd0fa7db211c12a3b8"}},
			},
		},
		{
			name:        "all",
			expectedIDs: []string{"bl", "wl", "other", "both"},
			policies: []policy.Policy{
				&policy.PrintersBulkAccessMode{Val: 2},
				&policy.PrintersBulkAllowlist{Val: []string{"both", "wl"}},
				&policy.PrintersBulkBlocklist{Val: []string{"both", "bl"}},
				&policy.PrintersBulkConfiguration{Val: &policy.PrintersBulkConfigurationValue{Url: "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/printers.json", Hash: "7a052c5e4f23c159668148df2a3c202bed4d65749cab5ecd0fa7db211c12a3b8"}},
			},
		},
		{
			name:        "unset",
			expectedIDs: []string{"bl", "wl", "other", "both"},
			policies: []policy.Policy{
				&policy.PrintersBulkAccessMode{Stat: policy.StatusUnset},
				&policy.PrintersBulkAllowlist{Val: []string{"both", "wl"}},
				&policy.PrintersBulkBlocklist{Val: []string{"both", "bl"}},
				&policy.PrintersBulkConfiguration{Val: &policy.PrintersBulkConfigurationValue{Url: "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/printers.json", Hash: "7a052c5e4f23c159668148df2a3c202bed4d65749cab5ecd0fa7db211c12a3b8"}},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			// Retrieve Printers seen by user.
			if err := conn.Exec(ctx, `	window.__printers = null;
										chrome.autotestPrivate.getPrinterList(function(printers) {
			    							window.__printers = printers;
										});`); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}
			if err := conn.WaitForExpr(ctx, "window.__printers !== null"); err != nil {
				s.Fatal("Failed to wait for non null printers: ", err)
			}
			printers := make([]map[string]string, 0)
			if err := conn.Eval(ctx, `window.__printers`, &printers); err != nil {
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
				s.Fatal(errors.Errorf("unexpected ids (-want +got): %v", diff))
			}
		})
	}
}
