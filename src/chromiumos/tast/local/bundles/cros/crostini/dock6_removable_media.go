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

package crostini

import (
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"context"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock6RemovableMedia,
		Desc:         "Test flash drive, mouse, keyboard, webcam while connect/disconnect via a Dock",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted", //Boot-up and Sign-In to the device
		Timeout:      10 * time.Minute,
		Vars:         utils.GetInputVars(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func Dock6RemovableMedia(ctx context.Context, s *testing.State) {
	// set up
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	uc, err := utils.NewUsbController(ctx, s)
	if err != nil {
		s.Fatal("Failed to create usb controller: ", err)
	}

	s.Logf("Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := Dock6RemovableMedia_Step2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := Dock6RemovableMedia_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - Connect any of this (Flash Drive /Mouse /Keyboard /Webcam onto (Dock station)
	if err := Dock6RemovableMedia_Step4(ctx, s, tconn, uc); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5 - copy files
	if err := Dock6RemovableMedia_Step5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// step 6 - disconnect peripherals
	if err := Dock6RemovableMedia_Step6(ctx, s, uc); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	return

}

// 1) Boot-up and Sign-In to the device
// 2) Connect ext-display to (Docking station)
func Dock6RemovableMedia_Step2(ctx context.Context, s *testing.State) error {

	s.Logf("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to connect ext-display to docking station: ")
	}

	return nil
}

// 3) Connect (Docking station) to Chromebook
func Dock6RemovableMedia_Step3(ctx context.Context, s *testing.State) error {

	s.Logf("Step 3 - Connect (Docking station) to Chromebook")

	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in docking to chromebook: ")
	}

	return nil
}

// 4) Connect any of this (Flash Drive /Mouse /Keyboard /Webcam onto (Dock station)
// 4) Make sure the peripheral connected onto (Dock station) work without any issue (Check for Mouse/Keyboard responsive and overall performance see any delay)
func Dock6RemovableMedia_Step4(ctx context.Context, s *testing.State, tconn *chrome.TestConn, uc *utils.UsbController) error {

	s.Logf("Step 4 - Connect any of this (Flash Drive /Mouse /Keyboard /Webcam onto (Dock station)")

	if err := uc.ControlUsbs(ctx, s, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in peripherals to station: ")
	}

	// verify keyboard
	if err := utils.VerifyKeyboard(ctx, s); err != nil {
		return errors.Wrap(err, "Failed to verify keyboard: ")
	}

	// verify mouse
	if err := utils.VerifyMouse(ctx, s); err != nil {
		return errors.Wrap(err, "Failed to verify mouse: ")
	}

	return nil
}

// refer to image_quick_view.go
// 5) Open (File Manager App) and Copy and Paste some files to "Flash Drive" (Make sure use the mouse and keyboard)
// 5) Make sure "Files" are successfully copied and able to open without any issue
func Dock6RemovableMedia_Step5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Logf("Step 5 - Copy files from usb to download path")

	previewImageFile := "Lighthouse.jpg"
	previewImageDimensions := "1024 x 768"

	// to get usb path, sth like /media/removable/{$usbName}
	getUsbPath := testexec.CommandContext(ctx, "sh", "-c", "sudo lsblk -l -o mountpoint | grep removable")
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
		s.Fatal("Launching the Files App failed: ", err)
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
		s.Fatal("Failed to view image preview information: ", err)
	}
	return nil
}

// 6) Disconnect each peripheral from step: #4 individually to (ensure no issue)
// 6) Make sure no crash or freeze on the device and "Touchpad and Keyboard" still work without issue
func Dock6RemovableMedia_Step6(ctx context.Context, s *testing.State, uc *utils.UsbController) error {

	s.Logf("Step 6 - Disconnect each peripheral from step: #4 individually to (ensure no issue)")

	if err := uc.ControlUsbs(ctx, s, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "Failed to unplug usb from docking station: ")
	}

	// verify keyboard
	if err := utils.VerifyKeyboard(ctx, s); err != nil {
		return errors.Wrap(err, "Failed to verify keyboard: ")
	}

	return nil
}
