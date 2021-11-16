// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/strcmp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrintersBulkAccessModeManagedGuestSession,
		Desc: "Verify behavior of default PrintersBulkAccessMode policy on Managed Guest Session",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func PrintersBulkAccessModeManagedGuestSession(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
		mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{
			&policy.PrintersBulkConfiguration{Val: &policy.PrintersBulkConfigurationValue{
				Url:  "https://storage.googleapis.com/chromiumos-test-assets-public/enterprise/printers.json",
				Hash: "7a052c5e4f23c159668148df2a3c202bed4d65749cab5ecd0fa7db211c12a3b8",
			}},
		}),
	)
	if err != nil {
		s.Error("Failed to start Chrome on Signin screen with default MGS account: ", err)
	}
	defer mgs.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	expectedIDs := []string{"bl", "wl", "other", "both"}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

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
		s.
			Error("Received response contains duplicates")
	}

	if diff := strcmp.SameList(expectedIDs, ids); diff != "" {
		s.Error(errors.Errorf("unexpected IDs (-want +got): %v", diff))
	}
}
