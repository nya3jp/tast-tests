// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnicornAddAccount,
		Desc:         "Veirfy that unicorn account cannot add a non-EDU secondary android account",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		VarDeps: []string{"geller.parentUser", "geller.parentPassword", "arc.parentPassword"},
		Fixture: "familyLinkUnicornArcLogin",
	})
}

func UnicornAddAccount(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn

	st, err := arc.GetState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get ARC state: ", err)
	}
	if st.Provisioned {
		s.Log("ARC is already provisioned. Skipping the Play Store setup")
	} else {
		if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to optin to Play Store and Close: ", err)
		}
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			} else if err := a.PullFile(ctx, "/sdcard/window_dump.xml",
				filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := openAndroidSettings(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to Open ARC Settings: ", err)
	}

	s.Log("Add non-EDU ARC account and verify")
	if err := addAndroidAccount(ctx, d, tconn, s); err != nil {
		s.Fatal("Failed to Add Account: ", err)
	}

}

// openAndroidSettings opens the ARC Settings Page from Chrome Settings.
func openAndroidSettings(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "apps", ui.Exists(playStoreButton)); err != nil {
		return errors.Wrap(err, "failed to launch apps settings page")
	}

	if err := uiauto.Combine("Open Android Settings",
		ui.FocusAndWait(playStoreButton),
		ui.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open ARC settings page")
	}
	return nil
}

// addAndroidAccount adds a second ARC account from ARC Settings->Accounts Screen.
func addAndroidAccount(ctx context.Context, arcDevice *androidui.Device, tconn *chrome.TestConn, s *testing.State) error {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	ui := uiauto.New(tconn)
	gellerParentUser := s.RequiredVar("geller.parentUser")
	gellerParentPass := s.RequiredVar("geller.parentPassword")
	parentPassword := s.RequiredVar("arc.parentPassword")

	// Scroll until Accounts is visible.
	scrollLayout := arcDevice.Object(androidui.ClassName(scrollClassName),
		androidui.Scrollable(true))
	accounts := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, 10*time.Second); err == nil {
		scrollLayout.ScrollTo(ctx, accounts)
	}
	if err := accounts.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Accounts")
	}

	addAccount := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Add account"), androidui.Enabled(true))
	if err := addAccount.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed finding Add account")
	}
	if err := addAccount.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Add account")
	}

	// Click on Google button which appears only on tablet flow.
	gaiaButton := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)Google"))
	if err := gaiaButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("gaiaButton doesn't exists: ", err)
	} else if err := gaiaButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Google")
	}

	parentPasswordText := nodewith.Name("Parent password").Role(role.TextField)
	s.Log("Clicking the parent password text field")
	if err := ui.LeftClick(parentPasswordText)(ctx); err != nil {
		return errors.Wrap(err, "failed to click parent password")
	}

	s.Log("Setting up keyboard")
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := kb.Type(ctx, parentPassword+"\n"); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	nextButton := nodewith.Name("Next").Role(role.Button)
	enterSchoolEmailText := nodewith.Name("School email").Role(role.TextField)
	if err := uiauto.Combine("Clicking next",
		ui.WaitUntilExists(nextButton),
		ui.LeftClickUntil(nextButton, ui.Exists(enterSchoolEmailText)))(ctx); err != nil {
		return errors.Wrap(err, "failed to click Next button")
	}

	if err := ui.LeftClick(enterSchoolEmailText)(ctx); err != nil {
		return errors.Wrap(err, "failed to click school email text field")
	}

	s.Log("Typing Geller parent (non-EDU) email")
	if err := kb.Type(ctx, gellerParentUser+"\n"); err != nil {
		return errors.Wrap(err, "failed to type Geller parent email")
	}
	schoolPasswordText := nodewith.Name("School password").Role(role.TextField)
	if err := uiauto.Combine("Clicking school account password text field",
		ui.WaitUntilExists(schoolPasswordText), ui.LeftClick(schoolPasswordText))(ctx); err != nil {
		return errors.Wrap(err, "failed to click school account password text field")
	}

	s.Log("Typing Geller parent (non-EDU) password")
	if err := kb.Type(ctx, gellerParentPass+"\n"); err != nil {
		return errors.Wrap(err, "failed to type Geller parent password")
	}

	if err := ui.WaitUntilExists(nodewith.Name("Can’t add account").Role(role.Heading))(ctx); err != nil {
		return errors.Wrap(err, "failed to detect can't add acccount error message")
	}
	return nil
}
