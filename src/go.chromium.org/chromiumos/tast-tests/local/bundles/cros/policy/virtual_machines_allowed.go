// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/fixture"
	"go.chromium.org/chromiumos/tast-tests/common/policy"
	"go.chromium.org/chromiumos/tast-tests/common/policy/fakedms"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/crostini/ui/settings"
	"go.chromium.org/chromiumos/tast-tests/local/policyutil"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualMachinesAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that installing Crostini is allowed only when VirtualMachinesAllowed policy is enabled",
		Contacts: []string{
			"janagrill@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
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
