// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	/*
		"chromiumos/tast/common/perf"
		"chromiumos/tast/local/chrome"
		"chromiumos/tast/local/chrome/ash"
		"chromiumos/tast/local/chrome/uiauto"
		"chromiumos/tast/local/chrome/uiauto/browser"
		"chromiumos/tast/local/chrome/uiauto/faillog"
		"chromiumos/tast/local/chrome/uiauto/nodewith"
		"chromiumos/tast/local/chrome/uiauto/ossettings"
		"chromiumos/tast/local/chrome/uiauto/role"
	*/
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionRestoreAfterColdBoot,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test session restore after DUT is rebooted",
		Contacts: []string{
			"abhijeet@igalia.com",
			"lacros-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome", "lacros", "reboot"},
	})
}

func SessionRestoreAfterColdBoot(ctx context.Context, s *testing.State) {

	// Login to a device and enable a FullRestore feature
	// Launch the browser and keep it open so that it could be restored afrer reboot
	/*
		func() {
			cr, err := chrome.New(ctx, chrome.EnableFeatures("FullRestore"))
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(ctx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect Test API: ", err)
			}

			// Open browser.
			// The opened browser is not closed before reboot so that it could be restored after reboot.
			if _, err = browser.Launch(ctx, tconn, cr, "https://abc.xyz"); err != nil {
				s.Fatal("Failed to launch browser: ", err)
			}

			// Open OS settings to set the 'Always restore' setting.
			if _, err = ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Link)); err != nil {
				s.Fatal("Failed to launch Apps Settings: ", err)
			}

			if err := uiauto.Combine("set 'Always restore' Settings",
				uiauto.New(tconn).LeftClick(nodewith.Name("Restore apps on startup").Role(role.PopUpButton)),
				uiauto.New(tconn).LeftClick(nodewith.Name("Always restore").Role(role.ListBoxOption)))(ctx); err != nil {
				s.Fatal("Failed to set 'Always restore' Settings: ", err)
			}

			// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
			// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
			// Therefore, sleep 3 seconds here.
			testing.Sleep(ctx, 3*time.Second)

		}()
	*/
	// Reboot the DUT
	func() {
		d := s.DUT()

		if err := d.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}
	}()

	// Note down the time before entering into user session
	// Note down the time after  browser is restored.
	/*
		func() {
			start := time.Now()
			cr, err := chrome.New(ctx,
				// Set not to clear the notification after restore.
				// By default, On startup is set to ask every time after reboot
				// and there is an alertdialog asking the user to select whether to restore or not.
				chrome.RemoveNotification(false),
				chrome.EnableFeatures("FullRestore"),
				chrome.EnableRestoreTabs(),
				chrome.KeepState())
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(ctx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect Test API: ", err)
			}

			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// Confirm that the browser is restored.
			if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.WindowType == ash.WindowTypeBrowser }); err != nil {
				s.Fatal("Failed to restore browser: ", err)
			}

			loadTime := time.Since(start)
			pv := perf.NewValues()
			pv.Set(perf.Metric{
				Name:      "session.restore.afterColdBoot",
				Unit:      "seconds",
				Direction: perf.SmallerIsBetter,
			}, loadTime.Seconds())
		}()
	*/
}
