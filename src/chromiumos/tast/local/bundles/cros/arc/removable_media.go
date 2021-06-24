// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemovableMedia,
		Desc: "Verifies ARC removable media integration is working",
		Contacts: []string{
			"hashimoto@chromium.org", // original author
			"hidehiko@chromium.org",  // Tast port author
			"arc-storage@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"capybara.jpg"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"ui.gaiaPoolDefault"},
	})
}

func RemovableMedia(ctx context.Context, s *testing.State) {
	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to PlayStore and Close
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	removablemedia.RunTest(ctx, s, a, "capybara.jpg")

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	ui := uiauto.New(tconn)
	externalStoragePreferenceButton := nodewith.Name("External storage preferences").Role(role.Link)
	myDiskButton := nodewith.Name("MyDisk").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "storage", ui.Exists(externalStoragePreferenceButton)); err != nil {
		s.Fatal("Failed to launch apps settings page: ", err)
	}

	if err := uiauto.Combine("Open Android Settings",
		ui.FocusAndWait(externalStoragePreferenceButton),
		ui.LeftClick(externalStoragePreferenceButton),
		ui.LeftClick(myDiskButton),
	)(ctx); err != nil {
		s.Fatal("Failed to Open Android Settings : ", err)
	}
}
