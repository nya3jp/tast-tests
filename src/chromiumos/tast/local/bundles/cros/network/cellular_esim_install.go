// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CellularESimInstall,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the add eSIM profile via activation code flow in the success and failure cases",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_test_esim"},
		Fixture:      "cellular",
		Timeout:      9 * time.Minute,
	})
}

func CellularESimInstall(ctx context.Context, s *testing.State) {
	euicc, slot, err := hermes.GetEUICC(ctx, true)
	if err != nil {
		s.Fatal("Failed to get test euicc: ", err)
	}

	// Remove any existing profiles on test euicc
	if err := euicc.DBusObject.Call(ctx, hermesconst.EuiccMethodResetMemory, 1).Err; err != nil {
		s.Fatal("Failed to reset test euicc: ", err)
	}
	defer euicc.DBusObject.Call(ctx, hermesconst.EuiccMethodResetMemory, 1)
	s.Log("Reset test euicc completed")

	if err := euicc.DBusObject.Call(ctx, hermesconst.EuiccMethodUseTestCerts, true).Err; err != nil {
		s.Fatal("Failed to set use test cert on eUICC: ", err)
	}
	s.Log("Set to use test cert on euicc completed")

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	chromeOpts := []chrome.Option{
		chrome.EnableFeatures("UseStorkSmdsServerAddress"),
	}
	if slot == 1 {
		s.Log("Append CellularUseSecondEuicc feature flag")
		chromeOpts = append(chromeOpts, chrome.EnableFeatures("CellularUseSecondEuicc"))
	}

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	mdp, err := ossettings.OpenMobileDataSubpage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to open mobile data subpage: ", err)
	}

	if err := waitUntilRefreshProfileCompletes(ctx, tconn); err != nil {
		s.Fatal("Failed to wait until refresh profile complete: ", err)
	}

	activationCode, cleanupFunc, err := stork.FetchStorkProfile(ctx)
	if err != nil {
		s.Fatal("Failed to fetch Stork profile: ", err)
	}
	defer cleanupFunc(ctx)

	s.Log("Fetched Stork profile with activation code: ", activationCode)

	// Use an incorrect activation code.
	var couldNotInstallProfileText = nodewith.NameContaining("Couldn't install eSIM profile").Role(role.StaticText)
	var incorrectActivationCode = string(activationCode) + "wrong"
	if err := addESimWithActivationCode(ctx, tconn, incorrectActivationCode); err != nil {
		s.Fatal("Failed to add incorrect activation code: ", err)
	}
	if err := uiauto.Combine("Exit add cellular eSIM flow after using incorrect activation code",
		mdp.WithTimeout(3*time.Minute).WaitUntilExists(couldNotInstallProfileText),
		mdp.LeftClick(ossettings.DoneButton.Focusable()),
	)(ctx); err != nil {
		s.Fatal("Correct activation code user journey fails: ", err)
	}

	// Use a correct activation code.
	var networkAddedText = nodewith.NameContaining("Network added").Role(role.StaticText)
	if err := addESimWithActivationCode(ctx, tconn, string(activationCode)); err != nil {
		s.Fatal("Failed to add correct activation code: ", err)
	}
	if err := uiauto.Combine("Exit add cellular eSIM flow after using correct activation code",
		mdp.WithTimeout(3*time.Minute).WaitUntilExists(networkAddedText),
		mdp.LeftClick(ossettings.DoneButton.Focusable()),
	)(ctx); err != nil {
		s.Fatal("Incorrect activation code user journey fails: ", err)
	}

	if err := verifyTestESimProfile(ctx, tconn); err != nil {
		s.Fatal("Failed to verify newly installed stork profile: ", err)
	}
}

func waitUntilRefreshProfileCompletes(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn).WithTimeout(1 * time.Minute)
	refreshProfileText := nodewith.NameContaining("This may take a few minutes").Role(role.StaticText)
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(refreshProfileText)(ctx); err == nil {
		if err := ui.WithTimeout(time.Minute).WaitUntilGone(refreshProfileText)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait until refresh profile complete")

		}
	}
	return nil
}

func addESimWithActivationCode(ctx context.Context, tconn *chrome.TestConn, activationCode string) error {
	if err := waitUntilRefreshProfileCompletes(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait until refresh profile complete")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	ui := uiauto.New(tconn).WithTimeout(1 * time.Minute)

	if err := ui.LeftClick(ossettings.AddCellularButton.Focusable())(ctx); err != nil {
		return errors.Wrap(err, "failed to click the Add Cellular Button")
	}

	var setupNewProfile = nodewith.NameContaining("Set up new profile").Role(role.Button).Focusable()
	if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(setupNewProfile)(ctx); err == nil {
		if err := ui.LeftClick(setupNewProfile)(ctx); err != nil {
			return errors.Wrap(err, "failed to click set up new profile button")
		}
	}

	var activationCodeInput = nodewith.NameRegex(regexp.MustCompile("Activation code")).Focusable().First()
	if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(activationCodeInput)(ctx); err != nil {
		return errors.Wrap(err, "failed to find activation code input field")
	}

	if err := ui.LeftClick(activationCodeInput)(ctx); err != nil {
		return errors.Wrap(err, "failed to find activation code input field")
	}

	if err := kb.Type(ctx, "LPA:"+activationCode); err != nil {
		return errors.Wrap(err, "could not type activation code")
	}

	if err := ui.LeftClick(ossettings.NextButton.Focusable())(ctx); err != nil {
		return errors.Wrap(err, "could not click Next button")
	}

	return nil
}

func verifyTestESimProfile(ctx context.Context, tconn *chrome.TestConn) error {
	if err := waitUntilRefreshProfileCompletes(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait until refresh profile complete")
	}

	ui := uiauto.New(tconn).WithTimeout(3 * time.Second)

	managedTestProfile := nodewith.NameRegex(regexp.MustCompile("^Network [0-9] of [0-9],.*"))
	// testProfileDetailButton is the finder for the "Test Profile" detail subpage arrow button in the mobile data page UI.
	var testProfileDetailButton = nodewith.ClassName("subpage-arrow").Role(role.Button).Ancestor(managedTestProfile.First())
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(testProfileDetailButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the newly installed test profile")
	}

	if err := ui.LeftClick(testProfileDetailButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click Test Profile detail button")
	}
	return nil
}
