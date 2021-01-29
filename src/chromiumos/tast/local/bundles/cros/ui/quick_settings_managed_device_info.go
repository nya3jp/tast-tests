// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: QuickSettingsManagedDeviceInfo,
		Desc: "Checks that the Quick Settings managed device info is displayed correctly",
		Contacts: []string{
			"leandre@chromium.org",
			"tbarzic@chromium.org",
			"kaznacheev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

const uiTimeout = 10 * time.Second

// QuickSettingsManagedDeviceInfo tests that the Quick Settings managed device info is displayed correctly.
func QuickSettingsManagedDeviceInfo(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.Auth(pre.Username, pre.Password, pre.GaiaID),
		chrome.DMSPolicy(fdms.URL),
		chrome.EnableFeatures("ManagedDeviceUIRedesign"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Check if management information is shown.
	managedBtn, err := ui.FindWithTimeout(ctx, tconn, quicksettings.ManagedInfoViewParams, uiTimeout)
	if err != nil {
		s.Fatal("Failed to find managed info button: ", err)
	}

	// Check if the information contains the managed domain name.
	if !strings.Contains(managedBtn.Name, "managedchrome.com") {
		s.Fatal("Managed info button did not show management domain name")
	}

	// Check if management information is clickable.
	if err := managedBtn.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click management information button: ", err)
	}
}
