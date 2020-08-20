// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
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
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"apps.LaunchHelpAppOnManagedDevice.enterprise_username", "apps.LaunchHelpAppOnManagedDevice.enterprise_password"},
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
			}, {
				Name:              "logged_in_stable",
				ExtraHardwareDeps: pre.AppsStableModels,
				Val:               false,
			}, {
				Name:              "logged_in_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Val:               false,
			},
		}})
}

// LaunchHelpAppOnManagedDevice verifies launching Showoff on a managed device.
func LaunchHelpAppOnManagedDevice(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("apps.LaunchHelpAppOnManagedDevice.enterprise_username")
	password := s.RequiredVar("apps.LaunchHelpAppOnManagedDevice.enterprise_password")

	isOOBE := s.Param().(bool)

	// TODO(b/161938620): Switch to fake DMS once crbug.com/1099310 is resolved.
	args := append([]chrome.Option(nil), chrome.Auth(username, password, "gaia-id"), chrome.GAIALogin(), chrome.ProdPolicy())
	if isOOBE {
		args = append(args, chrome.DontSkipOOBEAfterLogin(), chrome.ExtraArgs("--enable-features=HelpAppFirstRun"))
	}

	cr, err := chrome.New(
		ctx,
		args...,
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

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
