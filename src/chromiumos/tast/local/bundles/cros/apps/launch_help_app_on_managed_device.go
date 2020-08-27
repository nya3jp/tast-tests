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
	"chromiumos/tast/local/chrome/ui/faillog"
	policyPre "chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppOnManagedDevice,
		Desc: "Launch Help App on a managed device",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model(pre.AppsCriticalModels...)),
		Params: []testing.Param{
			{
				Name: "oobe",
				Val:  true,
			}, {
<<<<<<< HEAD   (fbcda1 camera: Try only resolutions options shown on UI)
				Name: "logged_in",
				Val:  false,
=======
				Name:              "oobe_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Val:               true,
			}, {
				Name:              "logged_in_stable",
				ExtraHardwareDeps: pre.AppsStableModels,
				Pre:               policyPre.User,
				Val:               false,
			}, {
				Name:              "logged_in_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Pre:               policyPre.User,
				Val:               false,
>>>>>>> CHANGE (ac1699 tast-tests: using fakedms to avoid enrolment recaptcha issue)
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
			chrome.Auth(policyPre.Username, policyPre.Password, policyPre.GaiaID),
			chrome.DMSPolicy(fdms.URL), chrome.DontSkipOOBEAfterLogin(),
			chrome.ExtraArgs("--enable-features=HelpAppFirstRun"),
		)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
	} else {
		cr = s.PreValue().(*policyPre.PreData).Chrome
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

		isPerkShown, err := helpapp.IsPerkShown(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to check perks visibility: ", err)
		}

		if isPerkShown {
			s.Error("Perks should not be shown on managed devices")
		}
	}
}
