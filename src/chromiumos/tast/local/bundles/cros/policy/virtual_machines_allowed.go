// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualMachinesAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that installing Crostini is allowed only when VirtualMachinesAllowed policy is enabled",
		Contacts: []string{
			"janagrill@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.VirtualMachinesAllowed{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func VirtualMachinesAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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
		// wantDialog expresses whether we expect the Crostini dialog to appear.
		wantDialog bool
	}{
		{
			name:       "enabled",
			value:      &policy.VirtualMachinesAllowed{Val: true},
			wantDialog: true,
		},
		{
			name:       "disabled",
			value:      &policy.VirtualMachinesAllowed{Val: false},
			wantDialog: false,
		},
		{
			name:       "unset",
			value:      &policy.VirtualMachinesAllowed{Stat: policy.StatusUnset},
			wantDialog: true,
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

			if _, err := settings.OpenLinuxInstaller(ctx, tconn, cr); err != nil {
				s.Fatal("Failed to open Linux installer: ", err)
			}

			ui := uiauto.New(tconn)
			crostiniDialogTitle := nodewith.Name("Set up Linux development environment").Role(role.StaticText)
			if param.wantDialog {
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
