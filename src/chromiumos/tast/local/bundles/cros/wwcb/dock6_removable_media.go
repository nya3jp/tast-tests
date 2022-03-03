// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
/***
#6 Removable Media Test over a Dock
Pre-Condition:
(Please note: Brand / Model number on test result)
1. External displays
2. Docking station
3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)
4. Flash drive /Mouse /Keyboard /Webcam
Procedure:
1) Boot-up and Sign-In to the device
2) Connect ext-display to (Docking station)
3) Connect (Docking station) to Chromebook
4) Connect any of this (Flash Drive /Mouse /Keyboard /Webcam onto (Dock station)
5) Open (File Manager App) and Copy and Paste some files to "Flash Drive" (Make sure use the mouse and keyboard)
6) Disconnect each peripheral from step: #4 individually to (ensure no issue)
Verification:
4) Make sure the peripheral connected onto (Dock station) work without any issue (Check for Mouse/Keyboard responsive and overall performance see any delay)
5) Make sure "Files" are successfully copied and able to open without any issue
6) Make sure no crash or freeze on the device and "Touchpad and Keyboard" still work without issue
***/

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock6RemovableMedia,
		Desc:         "Test flash drive, mouse, keyboard, webcam while connect/disconnect via a Dock",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars: []string{
			"WWCBIP",
			"DockingID",
			"1stExtDispID",
			"AllUSBID"},
		Pre:  chrome.LoggedIn(),
		Data: []string{"test_video.mp4", "test_pic.png"},
	})
}

func Dock6RemovableMedia(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")
	allUSBID := s.RequiredVar("AllUSBID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	// Copy files to downloads.
	testPng := "test_pic.png"
	testPngLocation := filepath.Join(filesapp.DownloadPath, testPng)
	if err := fsutil.CopyFile(s.DataPath(testPng), testPngLocation); err != nil {
		s.Fatalf("Failed to copy the golden audio file to %s: %s", testPngLocation, err)
	}
	defer os.Remove(testPngLocation)

	testVideo := "test_video.mp4"
	testVideoLocation := filepath.Join(filesapp.DownloadPath, testVideo)
	if err := fsutil.CopyFile(s.DataPath(testVideo), testVideoLocation); err != nil {
		s.Fatalf("Failed to copy the golden audio file to %s: %s", testVideoLocation, err)
	}
	defer os.Remove(testVideoLocation)

	cleanup, err := utils.InitFixtures(ctx)
	if err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer cleanup(ctx)

	// Create usb controller.
	uc, err := utils.NewUsbController(ctx, dockingID, allUSBID)
	if err != nil {
		s.Fatal("Failed to create usb controller: ", err)
	}

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// Step 2 - Connect ext-display to docking station.
	if err := dock6RemovableMediaStep2(ctx, extDispID1); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// Step 3 - Connect docking station to Chromebook.
	if err := dock6RemovableMediaStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// Step 4 - Connect (Flash Drive /Mouse /Keyboard /Webcam) onto docking station.
	if err := dock6RemovableMediaStep4(ctx, tconn, uc); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// Step 5 - Copy file to downloads, then open it to verify.
	if err := dock6RemovableMediaStep5(ctx, tconn, testPng); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// Step 6 - Disconnect peripherals and check Chromebook status is ok.
	if err := dock6RemovableMediaStep6(ctx, tconn, kb, uc, testVideo); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}
}

func dock6RemovableMediaStep2(ctx context.Context, extDispID1 string) error {
	testing.ContextLog(ctx, "Step 2 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock6RemovableMediaStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	if err := utils.VerifyExternalDisplay(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to verify external display is connected")
	}
	return nil
}

func dock6RemovableMediaStep4(ctx context.Context, tconn *chrome.TestConn, uc *utils.UsbController) error {
	testing.ContextLog(ctx, "Step 4 - Connect any of this (Flash Drive /Mouse /Keyboard /Webcam) onto Dock station")
	if err := uc.ControlUsbDevices(ctx, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to control usb devices")
	}
	if err := uc.VerifyUsbCount(ctx, true); err != nil {
		return errors.Wrap(err, "failed to verify usb devices are connected")
	}
	return nil
}

func dock6RemovableMediaStep5(ctx context.Context, tconn *chrome.TestConn, previewImageFile string) error {
	testing.ContextLog(ctx, "Step 5 - Copy file from downloads to usb, then open it to verify")

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch files")
	}
	defer files.Close(ctx)

	// Copy file from downloads to removable usb
	getUsbPathCmd := fmt.Sprint("sudo lsblk -l -o mountpoint | grep removable")
	output, err := testexec.CommandContext(ctx, "sh", "-c", getUsbPathCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to execute %s ", getUsbPathCmd)
	}
	usbPath := strings.TrimSpace(string(output))

	if err := testexec.CommandContext(ctx, "cp", filepath.Join(filesapp.DownloadPath, previewImageFile), usbPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to copy downloads to %s", usbPath)
	}

	usbName := filepath.Base(usbPath)
	previewImageDimensions := "1700 x 2200"
	openButton := nodewith.Name("Open").Role(role.Button)
	dimensionText := nodewith.Name(previewImageDimensions).Role(role.StaticText)

	// View image preview information of test image.
	if err := uiauto.Combine("View image preview information",
		files.OpenDir(usbName, filesapp.FilesTitlePrefix+usbName),
		files.WithTimeout(10*time.Second).WaitForFile(previewImageFile),
		files.SelectFile(previewImageFile),
		files.WithTimeout(10*time.Second).WaitUntilExists(openButton),
		files.OpenQuickView(previewImageFile),
		files.WithTimeout(10*time.Second).WaitUntilExists(dimensionText))(ctx); err != nil {
		return errors.Wrap(err, "failed to view image preview information")
	}
	return nil
}

func dock6RemovableMediaStep6(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, uc *utils.UsbController, testVideo string) error {
	testing.ContextLog(ctx, "Step 6 - Disconnect each usb devices then check chromebook status is ok")

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch files")
	}
	defer files.Close(ctx)

	// Unplug usb devices and verify.
	if err := uc.ControlUsbDevices(ctx, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to control usb devices")
	}
	if err := uc.VerifyUsbCount(ctx, false); err != nil {
		return errors.Wrap(err, "failed to verify usb devices are disconnected")
	}

	// Open test video and play.
	if err := uiauto.Combine("Open test video",
		files.OpenDownloads(),
		files.OpenFile(testVideo))(ctx); err != nil {
		return errors.Wrap(err, "failed to open test video")
	}
	defer apps.Close(ctx, tconn, apps.Gallery.ID)

	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, 5*time.Second); err != nil {
		return err
	}

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Play video in fullscreen",
		ui.LeftClick(nodewith.Name("Toggle fullscreen")),
		ui.LeftClick(nodewith.Name("Toggle play pause")))(ctx); err != nil {
		return errors.Wrap(err, "failed to play video in fullscreen")
	}

	// Let WWCB server to record video with camera then detect it.
	videoPath, err := utils.VideoRecord(ctx, "15", "chromebook")
	if err != nil {
		return errors.Wrap(err, "failed to record video")
	}

	if err := utils.DetectVideo(ctx, videoPath); err != nil {
		return errors.Wrap(err, "failed to detect video")
	}
	return nil
}
