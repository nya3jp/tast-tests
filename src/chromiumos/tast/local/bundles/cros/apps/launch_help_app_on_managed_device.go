// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	policyFixt "chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppOnManagedDevice,
		Desc: "Launch Help App on a managed device",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "oobe_stable",
				ExtraHardwareDeps: pre.AppsStableModels,
				Val:               true,
			}, {
				Name:              "oobe_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Val:               true,
				ExtraAttr:         []string{"informational"},
			}, {
				Name:              "logged_in_stable",
				ExtraHardwareDeps: pre.AppsStableModels,
				Fixture:           "chromePolicyLoggedIn",
				Val:               false,
			}, {
				Name:              "logged_in_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Fixture:           "chromePolicyLoggedIn",
				Val:               false,
				ExtraAttr:         []string{"informational"},
			},
		}})
}

// LaunchHelpAppOnManagedDevice verifies launching Showoff on a managed device.
func LaunchHelpAppOnManagedDevice(ctx context.Context, s *testing.State) {
	isOOBE := s.Param().(bool)

	var cr *chrome.Chrome
	if isOOBE {
		// Using fakedms and login
		// Start FakeDMS.
		tmpdir, err := ioutil.TempDir("", "fdms-")
		if err != nil {
			s.Fatal("Failed to create fdms temp dir: ", err)
		}
		defer os.RemoveAll(tmpdir)

		testing.ContextLogf(ctx, "FakeDMS starting in %s", tmpdir)
		fdms, err := fakedms.New(ctx, tmpdir)
		if err != nil {
			s.Fatal("Failed to start FakeDMS: ", err)
		}
		defer fdms.Stop(ctx)

		if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
			s.Fatal("Failed to write policies to FakeDMS: ", err)
		}

		cr, err = chrome.New(
			ctx,
			chrome.FakeLogin(chrome.Creds{User: policyFixt.Username, Pass: policyFixt.Password}),
			chrome.DMSPolicy(fdms.URL), chrome.DontSkipOOBEAfterLogin(),
			chrome.EnableFeatures("HelpAppFirstRun"),
		)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
	} else {
		cr = s.FixtValue().(*policyFixt.FixtData).Chrome
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// In OOBE stage, help app should not be launched on a managed device after login.
	if isOOBE {
		isAppLaunched, err := helpapp.Exists(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to verify help app existence: ", err)
		}

		if isAppLaunched {
			s.Error("Help app should not be launched after oobe on a managed device")
		}
	} else {
		// Once the help app has been launched, perks should not be shown on a managed device.
		if err := helpapp.Launch(ctx, tconn); err != nil {
			s.Fatal("Failed to launch help app: ", err)
		}

		// Check that loadTimeData is correctly set.
		loadTimeData, err := helpapp.GetLoadTimeData(ctx, cr)
		if err != nil {
			s.Fatal("Failed to get help app's load time data")
		}

		if !loadTimeData.IsManagedDevice {
			s.Error("Help app incorrectly set isManagedDevice to false")
		}

		// Check if perks tab is shown.
		isPerkShown, err := helpapp.IsTabShown(ctx, tconn, helpapp.PerksTab)
		if err != nil {
			s.Fatal("Failed to check perks visibility: ", err)
		}

		if isPerkShown {
			s.Error("Perks should not be shown on managed devices")
		}
	}
}
