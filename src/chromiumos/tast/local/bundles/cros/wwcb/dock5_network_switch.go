// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/***
#8 Wired/WiFi network switching over Dock
Pre-Condition:
(Please note: Brand / Model number on test result)
1. External displays
2. Docking station / Hub
3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)
4. Wired and WiFi connection (Router / Wireless Hub)

Procedure:
1) Boot-up and Sign-In to the device
2) Connect ext-display to (Docking station)
3) Connect (Docking station) to Chromebook
4) Connect wired Ethernet cable onto (Dock station or Hub)
5) Open Chrome Browser: www.YouTube.com and play any video
6) Disconnect Ethernet cable, and connect to WiFi
7) Repeat step: #5

Verification:
4)  Make sure (Quick Settings Menu) show "Ethernet" connection
HideAllNotifications
5)  Make sure video/audio playback without any issue
6)  Make sure (Quick Setting Menu) show "WiFi" connection
7)  Make sure video/audio playback without any issue
*/

// headphone pluging check command
//cras_test_client | grep *Headphone | grep yes
//(9e934263)      7:0        75 0.000000     yes              no  1619683090              HEADPHONE            2*Headphone

// check eth0
// Ethernet : ifconfig eth0 | grep inet
// wifi : ifconfig wlan0 | grep inet
//Output dev: acpd7219m98357: :1,2
// enable/disable wifi : ifconfig wlan0 up/down

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock5NetworkSwitch,
		Desc:         "Test wired/WiFi network switching when connecting/disconnecting over a Dock",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID", "EthernetID"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock5NetworkSwitch(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID := s.RequiredVar("1stExtDispID")
	ethernetID := s.RequiredVar("EthernetID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// Step 2 - Connect ext-display to docking station.
	if err := dock5NetworkSwitchStep2(ctx, extDispID); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// Step 3 - Connect docking station to chromebook.
	if err := dock5NetworkSwitchStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// Step 4 - Connect ethernet to docking station.
	if err := dock5NetworkSwitchStep4(ctx, ethernetID); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// Step 5 - Play YouTube then check playback.
	if err := dock5NetworkSwitchStep5(ctx, cr, tconn, kb); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// Step 6 - Disconnect ethernet from docking station.
	if err := dock5NetworkSwitchStep6(ctx, ethernetID); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	// Step 7 - Play YouTube then check playback.
	if err := dock5NetworkSwitchStep7(ctx, cr, tconn, kb); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}
}

func dock5NetworkSwitchStep2(ctx context.Context, extDispID string) error {
	testing.ContextLog(ctx, "Step 2 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock5NetworkSwitchStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify external display is connected")
	}
	return nil
}

func dock5NetworkSwitchStep4(ctx context.Context, ethernetID string) error {
	testing.ContextLog(ctx, "Step 4 - Connect ethernet to docking station")
	if err := utils.SwitchFixture(ctx, ethernetID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect ethernet")
	}
	if err := utils.VerifyEthernetStatus(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify ethernet is connected")
	}
	return nil
}

func dock5NetworkSwitchStep5(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 5 - Play YouTube then check playback")
	if err := playYouTubeThenCheck(ctx, cr, tconn, kb); err != nil {
		return errors.Wrap(err, "failed to play YouTube then check playback")
	}
	return nil
}

func dock5NetworkSwitchStep6(ctx context.Context, ethernetID string) error {
	testing.ContextLog(ctx, "Step 6 - Unplug ethernet, and connect to WiFi")
	if err := utils.SwitchFixture(ctx, ethernetID, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to disconnect ethernet")
	}
	if err := utils.VerifyEthernetStatus(ctx, false); err != nil {
		return errors.Wrap(err, "failed to verfiy ethernet is disconnected")
	}
	return nil
}

func dock5NetworkSwitchStep7(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	testing.ContextLog(ctx, "Step 7 - Play YouTube then check playback")
	if err := playYouTubeThenCheck(ctx, cr, tconn, kb); err != nil {
		return errors.Wrap(err, "failed to play YouTube then check playback")
	}
	return nil
}

// playYouTubeThenCheck plays YouTube in fullscreen then let WWCB server check playback.
func playYouTubeThenCheck(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	const (
		VideoTitle = "test video"
		YouTubeURL = "https://youtu.be/Znq6Q-AmCkA"
	)

	// Open YouTube web and wait for it.
	conn, err := cr.NewConn(ctx, YouTubeURL, browser.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "failed to open YouTube web")
	}
	defer conn.Close()

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && strings.Contains(w.Title, VideoTitle) && w.IsVisible == true
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for the window to be visible")
	}

	// Close YouTube window in the end.
	browser, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.WindowType == ash.WindowTypeBrowser
	})
	if err != nil {
		return errors.Wrap(err, "failed to find browser")
	}
	defer browser.CloseWindow(ctx, tconn)

	// Enter in fullscreen.
	fullscreenBtn := nodewith.Name("Full screen (f)").Role(role.Button)

	ui := uiauto.New(tconn)
	if err := ui.LeftClick(fullscreenBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click fullscreen button")
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == browser.ID && w.State == ash.WindowStateFullscreen
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for fullscreen")
	}

	// Let WWCB server record video then detect it.
	videoPath, err := utils.VideoRecord(ctx, "15", "chromebook")
	if err != nil {
		return errors.Wrap(err, "failed to video record")
	}

	if err := utils.DetectVideo(ctx, videoPath); err != nil {
		return errors.Wrap(err, "failed to detect video")
	}
	return nil
}
