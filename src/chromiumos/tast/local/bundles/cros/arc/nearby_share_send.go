// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	apkName        = "ArcShareTestApp.apk"
	apkPackageName = "org.chromium.arc.testapp.share"

	// Id of the check box for using text.
	checkBoxTextID = "checkBoxText"
	// Id of the check box for using the small sized file.
	checkBoxSmallFileID = "checkBoxSmallFile"
	// Id of the check box for using the medium sized file.
	checkBoxMediumFileID = "checkBoxMediumFile"
	// Id of the check box for using the large sized file.
	checkBoxLargeFileID = "checkBoxLargeFile"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NearbyShareSend,
		Desc: "Installs ARC share test app and share text/file to Nearby Share via Sharesheet",
		Contacts: []string{
			"alanding@chromium.org",
			"kyleshima@chromium.org",
			"phshah@chromium.org",
			"arc-app-dev@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{apkName},
		Params: []testing.Param{
			{
				Name:              "dataoffline_allcontacts_text",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOfflineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxTextID,
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_text_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOfflineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxTextID,
					TransferTimeout: nearbycommon.SmallFileTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_small_file",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxSmallFileID,
					TransferTimeout: nearbycommon.MediumFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.MediumFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.MediumFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_small_file_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxSmallFileID,
					TransferTimeout: nearbycommon.MediumFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.MediumFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.MediumFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_medium_file",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxMediumFileID,
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_medium_file_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxMediumFileID,
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_large_file",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxLargeFileID,
					TransferTimeout: nearbycommon.ExtraLargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.ExtraLargeFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.ExtraLargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_large_file_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxLargeFileID,
					TransferTimeout: nearbycommon.ExtraLargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.ExtraLargeFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.ExtraLargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_multiple_files",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxSmallFileID + "," + checkBoxMediumFileID,
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataoffline_allcontacts_multiple_files_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: nearbytestutils.TestData{
					Filename:        checkBoxSmallFileID + "," + checkBoxMediumFileID,
					TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
					TestTimeout:     nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
				},
				Timeout: nearbycommon.DetectionTimeout + nearbycommon.LargeFileOnlineTransferTimeout,
			},
		},
	})
}

func shareUIID(id string) ui.SelectorOption {
	return ui.ID(apkPackageName + ":id/" + id)
}

// shareUIClick sends a "Click" message to an UI Object.
// The UI Object is selected from opts, which are the selectors.
func shareUIClick(ctx context.Context, d *ui.Device, opts ...ui.SelectorOption) error {
	const (
		// Default ui timeout.
		uiTimeout = 5 * time.Second
	)

	obj := d.Object(opts...)
	if err := obj.WaitForExists(ctx, uiTimeout); err != nil {
		return err
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on widget element")
	}
	return nil
}

// NearbyShareSend tests sharing with a CrOS device as sender and Android device
// as receiver. The test uses a custom Android APK (ArcShareTestApp) to verify
// sharing text and/or small, medium, and large files from app to share target
// through the Android Sharesheet. It also verifies ARC Nearby Share cache files
// directory is cleaned after sharing is completed.
func NearbyShareSend(ctx context.Context, s *testing.State) {
	const (
		nearbyChipLabel    = "NEARBY"
		nearbySharingTitle = "Nearby Share"

		// Class name of text view.
		textViewClassName = "android.widget.TextView"
		// Class name of check box.
		checkBoxClassName = "android.widget.CheckBox"
		// Class name of button.
		buttonClassName = "android.widget.Button"
		// Class name of sharesheet bubble view.
		sharesheetBubbleViewClassName = "SharesheetBubbleView"

		// Id of the button for sharing text or file.
		shareButtonID = "share_button"

		// URL of Nearby target webview.
		nearbyURL = "chrome://nearby"

		// Directory name for hidden ARC Nearby Share cache files.
		arcCacheFilesDir = ".NearbyShare"

		// Default UI timeout.
		uiTimeout = 7 * time.Second
		// Timeout waiting for Nearby Share URL.
		webviewTimeout = 2 * time.Second
		// Wait time for detecting target device.
		targetWaitTime = 5 * time.Second
	)

	cr := s.FixtValue().(*nearbyshare.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyshare.FixtData).TestConn
	a := s.FixtValue().(*nearbyshare.FixtData).ARC

	s.Log("Installing APK")
	if err := a.Install(ctx, s.DataPath(apkName), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatal("Failed to install: ", err)
	}

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to verify VM status: ", err)
	}
	if isVMEnabled {
		s.Log("Granting permissions")

		// Android 11+ require granting MANAGE_EXTERNAL_STORAGE permission.
		if err := a.Command(ctx, "appops", "set", apkPackageName, "MANAGE_EXTERNAL_STORAGE", "allow").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed granting all files access permission: ", err)
		}
	}

	s.Log("Launching share test app")

	// Start app w/ fullscreen.
	if err := a.Command(ctx, "am", "start", "--windowingMode", "1", "-n", apkPackageName+"/.MainActivity").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	testData := s.Param().(nearbytestutils.TestData)

	for _, uiID := range strings.Split(string(testData.Filename), ",") {
		// Make sure the check box is available.
		testCheckBoxItem := d.Object(ui.ClassName(checkBoxClassName), shareUIID(uiID))
		if err := testCheckBoxItem.WaitForExists(ctx, uiTimeout); err != nil {
			s.Fatal("Failed to wait reading check box: ", err)
		}

		// Click the file sharing box.
		if err = shareUIClick(ctx, d, ui.ClassName(checkBoxClassName), shareUIID(uiID)); err != nil {
			s.Fatal("Failed to click text check box item: ", err)
		}
	}

	// Click to open the Android sharesheet.
	if err = shareUIClick(ctx, d, ui.ClassName(buttonClassName), shareUIID(shareButtonID)); err != nil {
		s.Fatal("Failed to share : ", err)
	}

	// For P, Nearby Share is using a text view with title.
	nearbyText := nearbySharingTitle
	nearbyClass := textViewClassName
	if isVMEnabled {
		// For R, Nearby Share is using a chip button.
		nearbyText = nearbyChipLabel
		nearbyClass = buttonClassName
	}

	// Make sure the Nearby Share button is available.
	testNearbyItem := d.Object(ui.ClassName(nearbyClass), ui.TextMatches(nearbyText))
	if err := testNearbyItem.WaitForExists(ctx, uiTimeout); err != nil {
		s.Fatal("Failed to wait while reading Nearby Share text view: ", err)
	}

	// Click to open the Nearby Share target.
	if err = shareUIClick(ctx, d, ui.ClassName(nearbyClass), ui.TextMatches(nearbyText)); err != nil {
		s.Fatal("Failed to select Nearby Share target: ", err)
	}

	// Make sure Nearby Share bubble view is available.
	ui := uiauto.New(tconn)
	bubbleView := nodewith.ClassName(sharesheetBubbleViewClassName).Role(role.Window)
	if err := ui.WaitUntilExists(bubbleView)(ctx); err != nil {
		s.Fatal("Failed to wait for sharesheet bubble view")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ok, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURLPrefix(nearbyURL))
		if err != nil {
			return testing.PollBreak(err)
		}
		if !ok {
			return errors.Errorf("no match target for %s", nearbyURL)
		}
		return nil
	}, &testing.PollOptions{Timeout: webviewTimeout}); err != nil {
		s.Fatalf("Failed to open %s: %v", nearbyURL, err)
	}
	// Extra wait time to detect target device.
	testing.Sleep(ctx, targetWaitTime)

	crosDisplayName := s.FixtValue().(*nearbyshare.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyshare.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyshare.FixtData).AndroidDeviceName

	s.Log("Starting payload send using Nearby Share on the CrOS device")

	// Connect to Nearby Sharing UI to control the transfer.
	sender, err := nearbyshare.ConnectToSharingUI(ctx, cr)
	if err != nil {
		s.Fatal("Failed to set up control over the Nearby Share send UI surface: ", err)
	}
	defer sender.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Starting high-visibility receiving on the Android device")
	testTimeout := testData.TestTimeout
	if err := androidDevice.ReceiveFile(ctx, crosDisplayName, androidDisplayName, true, testTimeout); err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}

	// Defer cancelling receiving if something goes wrong.
	var shareCompleted bool
	defer func() {
		if !shareCompleted {
			s.Log("Cancelling receiving")
			if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "after_sharing_failed.png")); err != nil {
				s.Log("Failed to capture a screenshot before cancelling receiving")
			}
			if err := androidDevice.CancelReceivingFile(ctx); err != nil {
				s.Error("Failed to cancel receiving after the share failed: ", err)
			}
			if err := androidDevice.AwaitSharingStopped(ctx, testTimeout); err != nil {
				s.Error("Failed waiting for the Android device to signal that sharing has finished: ", err)
			}
		}
	}()

	s.Log("Waiting for CrOS sender to detect Android receiver")
	if err := sender.SelectShareTarget(ctx, androidDisplayName, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	s.Log("Waiting for Android receiver to detect the incoming share from CrOS sender")
	if err := androidDevice.AwaitReceiverConfirmation(ctx, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("Failed waiting for the Android device to detect the share: ", err)
	}

	// Get the secure sharing token to confirm the share on Android.
	token, err := sender.ConfirmationToken(ctx)
	if err != nil {
		s.Fatal("Failed to get confirmation token: ", err)
	}

	s.Log("Accepting the share on the Android receiver")
	if err := androidDevice.AcceptTheSharing(ctx, token); err != nil {
		s.Fatal("Failed to accept the share on the Android device: ", err)
	}

	s.Log("Waiting for the Android receiver to signal that sharing has completed")
	if err := androidDevice.AwaitSharingStopped(ctx, testData.TransferTimeout); err != nil {
		s.Fatal("Failed waiting for the Android device to signal that sharing has finished: ", err)
	}
	shareCompleted = true

	// Verify ARC Nearby Share cache files directory is cleaned up.
	ownerID, err := cryptohome.UserHash(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	filePath := filepath.Join("/home/user", ownerID, arcCacheFilesDir)
	if stat, err := os.Stat(filePath); err == nil && stat.IsDir() {
		s.Fatalf("Directory path %s is still present", filePath)
	} else if !os.IsNotExist(err) {
		s.Fatalf("Failed to check if directory path %s exists: %v", filePath, err)
	}
}
