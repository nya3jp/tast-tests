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
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/shutil"
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
			"1stPeripheralID",
			"2ndPeripheralID",
			"3rdPeripheralID",
			"4thPeripheralID",
			"5thPeripheralID"},
		Pre: chrome.LoggedIn(),
	})
}

func Dock6RemovableMedia(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")
	peripheralID1 := s.RequiredVar("1stPeripheralID")
	peripheralID2 := s.RequiredVar("2ndPeripheralID")
	peripheralID3 := s.RequiredVar("3rdPeripheralID")
	peripheralID4 := s.RequiredVar("4thPeripheralID")
	peripheralID5 := s.RequiredVar("5thPeripheralID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := dock6RemovableMediaStep2(ctx, extDispID1); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}
	// step 3 - connect station to chromebook
	if err := dock6RemovableMediaStep3(ctx, dockingID); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}
	// step 4 - Connect any of this (Flash Drive /Mouse /Keyboard /Webcam onto (Dock station)
	if err := dock6RemovableMediaStep4(ctx, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}
	// step 5 - copy files to downloads
	if err := dock6RemovableMediaStep5(ctx, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}
	// step 6 - disconnect peripherals
	if err := dock6RemovableMediaStep6(ctx, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}
	return
}

func dock6RemovableMediaStep2(ctx context.Context, extDispID1 string) error {
	testing.ContextLog(ctx, "Step 2 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock6RemovableMediaStep3(ctx context.Context, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect (Docking station) to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	return nil
}

func dock6RemovableMediaStep4(ctx context.Context, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5 string) error {
	testing.ContextLog(ctx, "Step 4 - Connect any of this (Flash Drive /Mouse /Keyboard /Webcam onto (Dock station)")
	if err := utils.SwitchFixture(ctx, peripheralID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect peripheral")
	}

	// TODO-verify keyboard

	// TODO-verify mouse

	return nil
}

// dock6RemovableMediaStep5 refer to image_quick_view.go
func dock6RemovableMediaStep5(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Step 5 - Copy files from usb to download path")

	previewImageFile := "Lighthouse.jpg"
	previewImageDimensions := "1024 x 768"

	// to get usb path, sth like /media/removable/{$usbName}
	getUsbPath := testexec.CommandContext(ctx, "sudo lsblk -l -o mountpoint | grep removable")
	usbPath, err := getUsbPath.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(getUsbPath.Args))
	}

	// copy file from usb to "Downloads" folder
	copyFiles := testexec.CommandContext(ctx, "cp",
		filepath.Join(strings.TrimSpace(string(usbPath)), previewImageFile),
		filesapp.DownloadPath)

	if err = copyFiles.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(copyFiles.Args))
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "launching the Files App failed")
	}

	openButton := nodewith.Name("Open").Role(role.Button)
	dimensionText := nodewith.Name(previewImageDimensions).Role(role.StaticText)
	// View image preview information of test image.
	if err := uiauto.Combine("View image preview information",
		files.OpenDownloads(),
		files.WithTimeout(10*time.Second).WaitForFile(previewImageFile),
		files.SelectFile(previewImageFile),
		files.WithTimeout(10*time.Second).WaitUntilExists(openButton),
		files.OpenQuickView(previewImageFile),
		files.WithTimeout(10*time.Second).WaitUntilExists(dimensionText))(ctx); err != nil {
		return errors.Wrap(err, "failed to view image preview information")
	}
	return nil
}

func dock6RemovableMediaStep6(ctx context.Context, peripheralID1, peripheralID2, peripheralID3, peripheralID4, peripheralID5 string) error {
	testing.ContextLog(ctx, "Step 6 - Disconnect each peripheral")
	if err := utils.SwitchFixture(ctx, peripheralID1, "off", "0"); err != nil {
		return errors.Wrap(err, "failed to disconnect periepheral")
	}

	// TODO-verify keyboard

	return nil
}
