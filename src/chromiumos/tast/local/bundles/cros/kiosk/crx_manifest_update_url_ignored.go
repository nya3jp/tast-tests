// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CRXManifestUpdateURLIgnored,
		Desc: "Checks if CRXManifestUpdateURLIgnored policy is correctly reflected in update mechanism of extensions",
		Contacts: []string{
			"zubeil@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      7 * time.Minute, // Starting multiple extensions requires longer timeout.
	})
}

func CRXManifestUpdateURLIgnored(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		ignoreCrxURL     bool
		originalAppTitle string
		updatedAppTitle  string
		appID            string
		updateURL        string
	}{
		// The test uses three mock Extensions, with 2 versions each. This allows testing of the correct
		// update logic in accordance with KioskCRXManifestUpdateURLIgnored policy.

		// crx-File does not contain an update URL. Version should correspond to updateURL in policy.
		{
			ignoreCrxURL:     true,
			originalAppTitle: "Test extension #4",
			updatedAppTitle:  "Test extension #4",
			appID:            "kjecmldfmbflidigcdfdnegjgkgggoih",
			updateURL:        "https://storage.googleapis.com/extension_test/kjecmldfmbflidigcdfdnegjgkgggoih-update-7.xml",
		},
		{
			ignoreCrxURL:     true,
			originalAppTitle: "Test extension #4 (updated)",
			updatedAppTitle:  "Test extension #4 (updated)",
			appID:            "kjecmldfmbflidigcdfdnegjgkgggoih",
			updateURL:        "https://storage.googleapis.com/extension_test/kjecmldfmbflidigcdfdnegjgkgggoih-update-8.xml",
		},
		// Update URL in crx points to V1 of the app. Version should correspond to updateURL in policy.
		{
			ignoreCrxURL:     true,
			originalAppTitle: "Test extension #5",
			updatedAppTitle:  "Test extension #5",
			appID:            "fimgekdokgldflggeacgijngdienfdml",
			updateURL:        "https://storage.googleapis.com/extension_test/fimgekdokgldflggeacgijngdienfdml-update-9.xml",
		},
		{
			ignoreCrxURL:     true,
			originalAppTitle: "Test extension #5 (updated)",
			updatedAppTitle:  "Test extension #5 (updated)",
			appID:            "fimgekdokgldflggeacgijngdienfdml",
			updateURL:        "https://storage.googleapis.com/extension_test/fimgekdokgldflggeacgijngdienfdml-update-10.xml",
		},
		// Update URL in crx points to V2 of the app. It should only upgrade to V2 if policy is false.
		{
			ignoreCrxURL:     true,
			originalAppTitle: "Test extension #6",
			updatedAppTitle:  "Test extension #6",
			appID:            "epeagdmdgnhlibpbnhalblaohdhhkpne",
			updateURL:        "https://storage.googleapis.com/extension_test/epeagdmdgnhlibpbnhalblaohdhhkpne-update-11.xml",
		},
		{
			ignoreCrxURL:     true,
			originalAppTitle: "Test extension #6 (updated)",
			updatedAppTitle:  "Test extension #6 (updated)",
			appID:            "epeagdmdgnhlibpbnhalblaohdhhkpne",
			updateURL:        "https://storage.googleapis.com/extension_test/epeagdmdgnhlibpbnhalblaohdhhkpne-update-12.xml",
		},
		// Since policy is set to false, the App should upgrade automatically.
		{
			ignoreCrxURL:     false,
			originalAppTitle: "Test extension #6",
			updatedAppTitle:  "Test extension #6 (updated)",
			appID:            "epeagdmdgnhlibpbnhalblaohdhhkpne",
			updateURL:        "https://storage.googleapis.com/extension_test/epeagdmdgnhlibpbnhalblaohdhhkpne-update-11.xml",
		},
	} {
		launchKioskAndVerify(ctx, s, tc.ignoreCrxURL, tc.originalAppTitle, tc.updatedAppTitle, tc.appID, tc.updateURL)
	}
}

func launchKioskAndVerify(ctx context.Context, s *testing.State, ignoreCrxURL bool, originalAppTitle, updatedAppTitle, appID, updateURL string) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	accountID := "foo@bar.com"
	accountType := policy.AccountTypeKioskApp

	kioskPolicy := []policy.Policy{
		&policy.KioskCRXManifestUpdateURLIgnored{Val: ignoreCrxURL},
	}

	account := &policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			{
				AccountID:   &accountID,
				AccountType: &accountType,
				KioskAppInfo: &policy.KioskAppInfo{
					AppId:     &appID,
					UpdateUrl: &updateURL,
				},
			},
		},
	}

	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.CustomLocalAccounts(account),
		kioskmode.ExtraPolicies(kioskPolicy),
		kioskmode.ExtraChromeOptions(
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		),
	)

	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}

	defer kiosk.Close(ctx)

	if !openExtensionAndCheckTitleChange(ctx, s, cr, originalAppTitle, updatedAppTitle) {
		s.Fatal("Missmatch in Version")
	}
}

// openExtensionAndCheckTitleChange opens the kiosk app from login screen using the originalAppTitle and evaluates if the launched app has updatedAppTitle.
func openExtensionAndCheckTitleChange(ctx context.Context, s *testing.State, cr *chrome.Chrome, originalAppTitle, updatedAppTitle string) bool {
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// UI needs some time to be stable to interact, see also start_app_from_sign_in_screen.go
	// I was not able to find another stable way to interact with the UI.
	testing.Sleep(ctx, 3*time.Second)

	// Start syslog reader.
	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

	testing.ContextLog(ctx, "Opening Kiosk app from signin screen")
	kioskAppsBtn := nodewith.Name("Apps").ClassName("MenuButton")
	testExtensionButton := nodewith.Name(originalAppTitle).ClassName("MenuItemView")
	versionNode := nodewith.NameStartingWith("Version: ").First()

	if err := uiauto.Combine("Open TestExtension via menu",
		ui.WaitUntilExists(kioskAppsBtn),
		ui.LeftClick(kioskAppsBtn),
		ui.WaitUntilExists(testExtensionButton),
		ui.LeftClick(testExtensionButton),
	)(ctx); err != nil {
		s.Fatal("Failed to start extension: ", err)
	}

	// Wait for kiosk to start.
	if err := kioskmode.ConfirmKioskStarted(ctx, reader); err != nil {
		s.Fatal("There was a problem while checking chrome logs for Kiosk related entries: ", err)
	}

	// Wait for extension UI to be visible.
	if err := ui.WaitUntilExists(versionNode)(ctx); err != nil {
		s.Fatal("Failed to wait for extension to launch: ", err)
	}

	titleNode := nodewith.ClassName("RootView").Name(updatedAppTitle)
	matchesVersion, err := ui.IsNodeFound(ctx, titleNode)
	if err != nil {
		s.Fatal("Failed to check title node: ", err)
	}

	return matchesVersion
}
