// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserPrintersAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test behavior of UserPrintersAllowed policy: check if Add printer button is restricted based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-networking@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
		Fixture: fixture.ChromePolicyLoggedIn,
	})
}

func UserPrintersAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		name            string
		printersAllowed bool
		policy          *policy.UserPrintersAllowed
	}{
		{
			name:            "unset",
			printersAllowed: true,
			policy:          &policy.UserPrintersAllowed{Stat: policy.StatusUnset},
		},
		{
			name:            "not_allowed",
			printersAllowed: false,
			policy:          &policy.UserPrintersAllowed{Val: false},
		},
		{
			name:            "allowed",
			printersAllowed: true,
			policy:          &policy.UserPrintersAllowed{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}
			ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

			// Open printer settings.
			_, err = cr.NewConn(ctx, "chrome://os-settings/cupsPrinters")

			// Click on the "Add printer" button until the "Add a printer manually" dialog appears.
			// This way we check if it is possible to add new printers as the restriction state of
			// the button is not visible in the ui tree. DoDefaultUntil cannot be used as it will
			// ignore the restrictions on the button.
			addPrinterButton := nodewith.Name("Add printer").Role(role.Button).Onscreen()
			dialog := nodewith.Name("Add a printer manually").Onscreen().First()
			if err = ui.WaitUntilExists(addPrinterButton)(ctx); err != nil {
				s.Fatal("Failed to wait for the \"Add printer\" button to exist: ", err)
			}
			dialogExists := true
			err = ui.LeftClickUntil(addPrinterButton, ui.Exists(dialog))(ctx)
			if err != nil {
				if strings.Contains(errors.Unwrap(err).Error(), "click may not have been received yet") {
					dialogExists = false
				} else {
					s.Fatal("Failed to click on \"Add printer\" button: ", err)
				}
			}

			if dialogExists != param.printersAllowed {
				s.Fatalf("Unexpected printer dialog existence; want: %q, got: %q", param.printersAllowed, dialogExists)
			}
		})
	}
}
