// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package familylink is used for writing Family Link tests.
package familylink

import (
	"context"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
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

	ui := uiauto.New(tconn)

	s.Log("Launching the in-session Edu Coexistence flow")
	if err := addEduSecondaryAccount(ctx, cr, tconn, parentUser, parentPass, eduUser, eduPass); err != nil {
		s.Fatal("Failed to go through the in-session Edu Coexistence flow: ", err)
	}

	s.Log("Clicking next on the final page to wrap up")
	schoolAccountAddedHeader := nodewith.Name("School account added").Role(role.Heading)
	if err := uiauto.Combine("Clicking next button and wrapping up",
		ui.WaitUntilExists(schoolAccountAddedHeader),
		ui.LeftClickUntil(nodewith.Name("Next").Role(role.Button), ui.Gone(schoolAccountAddedHeader)))(ctx); err != nil {
		s.Fatal("Failed to click next button: ", err)
	}

	s.Log("Verifying the EDU secondary account added successfully")
	// There should be a "more actions" button to remove the EDU secondary account.
	moreActionsButton := nodewith.Name("More actions, " + eduUser).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to detect EDU secondary account: ", err)
	}

	// Switch to EDU account in Play Store.
	if err := switchPlayStoreAccount(ctx, d, tconn, s, eduUser); err != nil {
		s.Fatal("Failed to Switch Account: ", err)
	}

	// "No results found" should be shown.
	noResultsText := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.Text("No results found."))
	if err := noResultsText.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("No Thanks button doesn't exists: ", err)
	}
}

func addEduSecondaryAccount(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn,
	parentUser, parentPass, secondUser, secondPass string) error {

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	testing.ContextLog(ctx, "Checking logged in user is Family Link")
	if err := ui.Exists(nodewith.Name("This account is managed by Family Link").Role(role.Image))(ctx); err != nil {
		return errors.Wrap(err, "logged in user is not Family Link")
	}

	testing.ContextLog(ctx, "Launching the settings app")
	addSchoolAccountButton := nodewith.Name("Add school account").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(addSchoolAccountButton)); err != nil {
		return errors.Wrap(err, "failed to launch Account Manager page")
	}

	if err := ui.WithInterval(time.Second).LeftClickUntil(addSchoolAccountButton, ui.Exists(nodewith.Name("Parent password").Role(role.TextField)))(ctx); err != nil {
		return errors.Wrap(err, "failed to open in-session edu coexistence flow")
	}

	if err := familylink.NavigateEduCoexistenceFlow(ctx, cr, tconn, parentPass, secondUser, secondPass); err != nil {
		return errors.Wrap(err, "failed to navigate in-session Edu Coexistence flow")
	}

	return nil
}

// switchPlayStoreAccount switches between the ARC account in PlayStore.
func switchPlayStoreAccount(ctx context.Context, arcDevice *androidui.Device,
	tconn *chrome.TestConn, s *testing.State, username string) error {
	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	noThanksButton := arcDevice.Object(androidui.ClassName("android.widget.Button"),
		androidui.TextMatches("(?i)No thanks"))
	if err := noThanksButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("No Thanks button doesn't exists: ", err)
	} else if err := noThanksButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click No Thanks button")
	}

	avatarIcon := arcDevice.Object(androidui.ClassName("android.widget.FrameLayout"),
		androidui.DescriptionContains("Account and settings"))
	if err := avatarIcon.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Avatar Icon")
	}

	expandAccountButton := arcDevice.Object(androidui.ClassName("android.view.ViewGroup"), androidui.Clickable(true))
	if err := expandAccountButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("Expand account button doesn't exists: ", err)
	} else if err := expandAccountButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click expand account button")
	}

	accountNameButton := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.Text(username))
	if err := accountNameButton.WaitForExists(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to find Account Name")
	}
	if err := accountNameButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Account Name")
	}
	return nil
}
