// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemTrayItemsPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures animation smoothness of items in the system tray",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func SystemTrayItemsPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to load the top-row layout: ", err)
	}

	// Starts full screen recording via UI.
	screenRecordToggleButton := nodewith.HasClass("CaptureModeToggleButton").Name("Screen record")
	recordFullscreenToggleButton := nodewith.HasClass("CaptureModeToggleButton").Name("Record full screen")
	stopRecordButton := nodewith.HasClass("TrayBackgroundView").Name("Stop screen recording")
	recordTakenLabel := nodewith.HasClass("Label").Name("Screen recording taken")
	popupNotification := nodewith.Role(role.Window).HasClass("ash/message_center/MessagePopup")

	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Take video recording so that the shelf pod bounces up, then click on the shelf pod for it to fade out,
		// (at the same time notification counter tray item will do show animation), then we close the notification
		// for tray item to perform hide animation.
		if err := uiauto.Combine(
			"perform screen recording, then close the notification",
			// Enter screen capture mode.
			kb.AccelAction("Ctrl+Shift+"+topRow.SelectTask),
			ac.LeftClick(screenRecordToggleButton),
			ac.LeftClick(recordFullscreenToggleButton),
			kb.AccelAction("Enter"),
			// Records full screen for 2 seconds.
			uiauto.Sleep(2*time.Second),
			ac.LeftClick(stopRecordButton),
			// Checks if the screen record is taken.
			ac.WaitUntilExists(recordTakenLabel),
			// Close the notification so that tray item performs hide animation.
			ac.LeftClick(popupNotification),
			// Opening too many "Files" app window might cause the system hang on some device.
			// Thus, we close that window on every iteration.
			kb.AccelAction("Ctrl+W"),
		)(ctx); err != nil {
			return err
		}
		return nil
	},
		"Ash.StatusArea.TrayBackgroundView.BounceIn",
		"Ash.StatusArea.TrayBackgroundView.Hide",
		"Ash.StatusArea.TrayItemView.Show",
		"Ash.StatusArea.TrayItemView.Hide")),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
