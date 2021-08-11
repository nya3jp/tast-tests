// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualMachinesAllowed,
		Desc: "Verify that installing Crostini is allowed only when VirtualMachinesAllowed policy is enabled",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

func VirtualMachinesAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.VirtualMachinesAllowed
	}{
		{
			name:  "enabled",
			value: &policy.VirtualMachinesAllowed{Val: true},
		},
		{
			name:  "disabled",
			value: &policy.VirtualMachinesAllowed{Val: false},
		},
		{
			name:  "unset",
			value: &policy.VirtualMachinesAllowed{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Indicates whether the Crostini installer dialog should appear.
			dialogExpected := param.value.Stat != policy.StatusUnset && param.value.Val

			// Trigger the Crostini installer.
			if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.runCrostiniInstaller)()", nil); err != nil {
				// If Crostini is not allowed, then this JS call
				// triggers an error, which is why we only consider the
				// error when Crostini is allowed by policy.
				if dialogExpected {
					s.Fatal("Failed to execute JS expression: ", err)
				}
			}

			ui := uiauto.New(tconn)
			crostiniDialogTitle := nodewith.Name("Set up Linux development environment").Role(role.StaticText)
			if dialogExpected {
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(crostiniDialogTitle)(ctx); err != nil {
					s.Error("Failed to find the Crostini installer title text: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(crostiniDialogTitle, 10*time.Second)(ctx); err != nil {
					s.Error("Crostini is installing and it should not be: ", err)
				}
			}
		})
	}
}
