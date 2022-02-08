// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EducoexistenceArc,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks ARC behavior for account added via in-session EDU Coexistence flow",
		Contacts:     []string{"anastasiian@chromium.org", "cros-families-eng+test@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + 5*time.Minute,
		VarDeps: []string{"arc.parentUser", "arc.parentPassword", "edu.user", "edu.password"},
		Fixture: "familyLinkUnicornArcLogin",
	})
}

func EducoexistenceArc(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	cr := s.FixtValue().(*familylink.FixtData).Chrome

	parentUser := s.RequiredVar("arc.parentUser")
	parentPass := s.RequiredVar("arc.parentPassword")
	eduUser := s.RequiredVar("edu.user")
	eduPass := s.RequiredVar("edu.password")

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}
	defer a.Close(ctx)
	defer a.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Launching the in-session Edu Coexistence flow")
	if err := familylink.AddEduSecondaryAccountWithOneParent(ctx, cr, tconn, parentUser, parentPass, eduUser, eduPass, true); err != nil {
		s.Fatal("Failed to complete the in-session Edu Coexistence flow: ", err)
	}

	// Switch to EDU account in Play Store.
	if err := arc.SwitchPlayStoreAccount(ctx, d, tconn, eduUser); err != nil {
		s.Fatal("Failed to Switch Account: ", err)
	}

	// "No results found" should be shown.
	noResultsText := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.Text("No results found."))
	if err := noResultsText.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Fatal("'No results found' message doesn't exists: ", err)
	}
}
