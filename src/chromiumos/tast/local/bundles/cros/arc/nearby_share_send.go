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
	"chromiumos/tast/common/android/ui"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbyfixture"
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

	// Large file generation additional UI timeout.
	largeFileTimeout = 40 * time.Second
	// nearbycommon.DetectionTimeout + additional test time needed for ARC setup.
	baseArcTestTime = nearbycommon.DetectionTimeout + 2*time.Minute
	// ExtraLargeFileOnlineTransferTimeout is for 30MB, add 7 more minutes for the
	// extra 70MBs needed to transfer an ArcShareTestApp large file.
	largeFileExtraBufferTime = 10 * time.Minute
)

type arcNearbyShareParams struct {
	Cancel       bool // Cancel the Share before Selecting Target (Discovery Page).
	CancelSelect bool // Cancel the Share after Selecting Target (Confirmation Page).
	TestData     nearbycommon.TestData
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NearbyShareSend,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Installs ARC share test app and share text/file to Nearby Share via Sharesheet",
		Contacts: []string{
			"alanding@chromium.org",
			"kyleshima@chromium.org",
			"phshah@chromium.org",
			"arc-app-dev@google.com",
		},
		Attr:         []string{"group:nearby-share-arc"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{apkName},
		Params: []testing.Param{
			{
				Name:              "dataoffline_noone_text",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOfflineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxTextID,
						TransferTimeout: nearbycommon.SmallFileTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.SmallFileTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:              "dataoffline_noone_text_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOfflineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxTextID,
						TransferTimeout: nearbycommon.SmallFileTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.SmallFileTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.SmallFileTransferTimeout,
			},
			{
				Name:              "dataonline_noone_small_file",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxSmallFileID,
						TransferTimeout: nearbycommon.MediumFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.MediumFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.MediumFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_small_file_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxSmallFileID,
						TransferTimeout: nearbycommon.MediumFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.MediumFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.MediumFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_medium_file",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_medium_file_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_large_file",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxLargeFileID,
						TransferTimeout: nearbycommon.ExtraLargeFileOnlineTransferTimeout + largeFileExtraBufferTime,
						TestTimeout:     baseArcTestTime + nearbycommon.ExtraLargeFileOnlineTransferTimeout + largeFileExtraBufferTime,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.ExtraLargeFileOnlineTransferTimeout + largeFileTimeout + largeFileExtraBufferTime,
			},
			{
				Name:              "dataonline_noone_large_file_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxLargeFileID,
						TransferTimeout: nearbycommon.ExtraLargeFileOnlineTransferTimeout + largeFileExtraBufferTime,
						TestTimeout:     baseArcTestTime + nearbycommon.ExtraLargeFileOnlineTransferTimeout + largeFileExtraBufferTime,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.ExtraLargeFileOnlineTransferTimeout + largeFileTimeout + largeFileExtraBufferTime,
			},
			{
				Name:              "dataonline_noone_multiple_files",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxSmallFileID + "," + checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_multiple_files_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					TestData: nearbycommon.TestData{
						Filename:        checkBoxSmallFileID + "," + checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_medium_file_cancel",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					Cancel: true,
					TestData: nearbycommon.TestData{
						Filename:        checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_medium_file_cancel_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					Cancel: true,
					TestData: nearbycommon.TestData{
						Filename:        checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_medium_file_cancel_select",
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					CancelSelect: true,
					TestData: nearbycommon.TestData{
						Filename:        checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
			{
				Name:              "dataonline_noone_medium_file_cancel_select_vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "nearbyShareDataUsageOnlineNoOneARCEnabled",
				Val: arcNearbyShareParams{
					CancelSelect: true,
					TestData: nearbycommon.TestData{
						Filename:        checkBoxMediumFileID,
						TransferTimeout: nearbycommon.LargeFileOnlineTransferTimeout,
						TestTimeout:     baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
					},
				},
				Timeout: baseArcTestTime + nearbycommon.LargeFileOnlineTransferTimeout,
			},
		},
	})
}

// NearbyShareSend tests sharing with a CrOS device as sender and Android device
// as receiver. The test uses a custom Android APK (ArcShareTestApp) to verify
// sharing text, small, medium, and/or large files from Android app to share target
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

		// Id of the button for sharing text or file.
		shareButtonID = "share_button"

		// Directory name under cryptohome for Web Share and ARC Nearby Share.
		shareCacheDir = "ShareCache"

		// Directory name for hidden ARC Nearby Share cache files.
		arcCacheFilesDir = ".NearbyShare"
	)

	cr := s.FixtValue().(*nearbyfixture.FixtData).Chrome
	tconn := s.FixtValue().(*nearbyfixture.FixtData).TestConn
	a := s.FixtValue().(*nearbyfixture.FixtData).ARC

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
	defer func() {
		s.Log("Closing the app")
		if err := a.Command(ctx, "am", "force-stop", apkPackageName).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to close the app: ", err)
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	testData := s.Param().(arcNearbyShareParams).TestData

	sharingFiles := false
	sharingLargeFile := false
	for _, uiID := range strings.Split(string(testData.Filename), ",") {
		if !sharingFiles && uiID != checkBoxTextID {
			// Mark sharing at least one file.
			sharingFiles = true
		}
		if uiID == checkBoxLargeFileID {
			sharingLargeFile = true
		}
		if err = shareUIClick(ctx, d, ui.ClassName(checkBoxClassName), shareUIID(uiID)); err != nil {
			s.Fatal("Failed to click text check box item: ", err)
		}

		// Verify that check box is checked.
		checked, err := d.Object(ui.ClassName(checkBoxClassName), shareUIID(uiID)).IsChecked(ctx)
		if err != nil {
			s.Fatal("Failed to verify check box is checked: ", err)
		} else if !checked {
			s.Fatal("Check box is not checked")
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

	// Click to open the Nearby Share target.
	if sharingLargeFile {
		if err = shareUIClickWithTimeout(ctx, d, largeFileTimeout, ui.ClassName(nearbyClass), ui.TextMatches(nearbyText)); err != nil {
			s.Fatal("Failed to select Nearby Share target: ", err)
		}
	} else {
		if err = shareUIClick(ctx, d, ui.ClassName(nearbyClass), ui.TextMatches(nearbyText)); err != nil {
			s.Fatal("Failed to select Nearby Share target: ", err)
		}
	}

	crosDisplayName := s.FixtValue().(*nearbyfixture.FixtData).CrOSDeviceName
	androidDevice := s.FixtValue().(*nearbyfixture.FixtData).AndroidDevice
	androidDisplayName := s.FixtValue().(*nearbyfixture.FixtData).AndroidDeviceName

	s.Log("Starting payload send using Nearby Share on the CrOS device")

	// Connect to Nearby Sharing UI to control the transfer.
	sender, err := nearbyshare.ConnectToSharingUI(ctx, cr)
	if err != nil {
		s.Fatal("Failed to set up control over the Nearby Share send UI surface: ", err)
	}
	defer sender.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ownerID, err := cryptohome.UserHash(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	filePath := filepath.Join("/home/user", ownerID, shareCacheDir, arcCacheFilesDir)
	if sharingFiles {
		// Verify ARC Nearby Share cache files directory is created when sharing files.
		if _, err := os.Stat(filePath); err != nil {
			s.Fatalf("Directory path %s does not exist", filePath)
		}
	}

	s.Log("Starting high-visibility receiving on the Android device")
	testTimeout := testData.TestTimeout
	if err := androidDevice.ReceiveFile(ctx, crosDisplayName, androidDisplayName, true, testTimeout); err != nil {
		s.Fatal("Failed to start receiving on Android: ", err)
	}

	coreui := uiauto.New(tconn)
	var shareCancelled bool
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
			if !shareCancelled {
				cancel := nodewith.Name("Cancel").Role(role.Button)
				close := nodewith.Name("Close").Role(role.Button).ClassName("action-button")
				if err := uiauto.Combine("find and click cancel button",
					uiauto.IfSuccessThen(coreui.WithTimeout(5*time.Second).WaitUntilExists(cancel), coreui.LeftClick(cancel)),
					uiauto.IfSuccessThen(coreui.WithTimeout(5*time.Second).WaitUntilExists(close), coreui.LeftClick(close)),
				)(ctx); err != nil {
					s.Error("Failed to click the 'Cancel' button: ", err)
				}
			}
		}
	}()

	if s.Param().(arcNearbyShareParams).Cancel {
		s.Log("Cancel before Selecting Target")
		if err := sender.Cancel(ctx); err != nil {
			s.Fatal("Cancel failed: ", err)
		}
		shareCancelled = true
		shareButton := d.Object(ui.ClassName(buttonClassName), ui.TextMatches("(?i)"+"SHARE"), ui.Enabled(true))
		if err := shareButton.WaitForExists(ctx, 10*time.Second); err != nil {
			s.Fatal("SHARE button doesn't exist: ", err)
		}
		if err := verifyCacheCleared(ctx, filePath); err != nil {
			s.Fatalf("Verifying %s failed: ", filePath)
		}
		return
	}

	s.Log("Waiting for CrOS sender to detect Android receiver")
	if err := sender.SelectShareTarget(ctx, androidDisplayName, nearbycommon.DetectShareTargetTimeout); err != nil {
		s.Fatal("CrOS device failed to select Android device as a receiver and start the transfer: ", err)
	}

	if s.Param().(arcNearbyShareParams).CancelSelect {
		s.Log("Cancel after selecting Target")
		if err := sender.CancelSelect(ctx); err != nil {
			s.Fatal("Cancel failed: ", err)
		}
		shareCancelled = true
		shareButton := d.Object(ui.ClassName(buttonClassName), ui.TextMatches("(?i)"+"SHARE"), ui.Enabled(true))
		if err := shareButton.WaitForExists(ctx, 10*time.Second); err != nil {
			s.Fatal("SHARE button doesn't exist: ", err)
		}
		if err := verifyCacheCleared(ctx, filePath); err != nil {
			s.Fatal("Verifying Cache Dir failed: ", err)
		}
		return
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

	if err := verifyCacheCleared(ctx, filePath); err != nil {
		s.Fatalf("Verifying %s failed", filePath)
	}

}

// shareUIID selects the UI element based on ID string.
func shareUIID(id string) ui.SelectorOption {
	return ui.ID(apkPackageName + ":id/" + id)
}

// shareUIClickWithTimeout sends a "Click" message to an UI object after verifying
// it exists and is enabled within |t| timeout duration.
// The UI object is selected from opts, which are the selectors.
func shareUIClickWithTimeout(ctx context.Context, d *ui.Device, t time.Duration, opts ...ui.SelectorOption) error {
	obj := d.Object(opts...)
	if err := obj.WaitForExists(ctx, t); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		enabled, err := obj.IsEnabled(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if !enabled {
			return errors.New("widget element is not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: t}); err != nil {
		return errors.Wrap(err, "failed to verify if widget element is enabled")
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on widget element")
	}
	return nil
}

// shareUIClick calls shareUIClickWithTimeout using the default timeout duration.
func shareUIClick(ctx context.Context, d *ui.Device, opts ...ui.SelectorOption) error {
	const (
		// Default UI timeout.
		uiTimeout = 10 * time.Second
	)

	return shareUIClickWithTimeout(ctx, d, uiTimeout, opts...)
}

// verifyCacheCleared checks that cache files directory is cleaned up for files.
// For text only, verify cache files directory does not exist.
func verifyCacheCleared(ctx context.Context, filePath string) error {
	if stat, err := os.Stat(filePath); err == nil && stat.IsDir() {
		errors.Wrapf(err, "directory path %s is still present", filePath)
	} else if !os.IsNotExist(err) {
		errors.Wrapf(err, "failed to check if directory path %s exists", filePath)
	}
	return nil
}
