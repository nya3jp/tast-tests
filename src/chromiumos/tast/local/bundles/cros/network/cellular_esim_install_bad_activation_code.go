// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CellularESimInstallBadActivationCode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the add eSIM profile via activation code flow in the success and failure cases",
		Contacts: []string{
			"hsuregan@google.com",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_test_esim"},
		Fixture:      "cellular",
		Timeout:      9 * time.Minute,
	})
}

func CellularESimInstallBadActivationCode(ctx context.Context, s *testing.State) {
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

	if err := ossettings.WaitUntilRefreshProfileCompletes(ctx, tconn); err != nil {
		s.Fatal("Failed to wait until refresh profile complete: ", err)
	}

	activationCode, cleanupFunc, err := stork.FetchStorkProfile(ctx)
	if err != nil {
		s.Fatal("Failed to fetch Stork profile: ", err)
	}
	defer cleanupFunc(ctx)

	s.Log("Fetched Stork profile with activation code: ", activationCode)

	s.Log("Flow 1: Use an incorrect activation code for an eSIM profile that does not require a confirmation code")
	var couldNotInstallProfileText = nodewith.NameContaining("Couldn't install eSIM profile").Role(role.StaticText)
	var incorrectActivationCode = string(activationCode) + "wrong"
	if err := ossettings.AddESimWithActivationCode(ctx, tconn, incorrectActivationCode); err != nil {
		s.Fatal("Failed to add esim profile with incorrect activation code: ", err)
	}
	if err := uiauto.Combine("Exit add cellular eSIM flow after using incorrect activation code",
		mdp.WithTimeout(3*time.Minute).WaitUntilExists(couldNotInstallProfileText),
		mdp.LeftClick(ossettings.DoneButton.Focusable()),
	)(ctx); err != nil {
		s.Fatal("Incorrect activation code user journey fails: ", err)
	}
}
