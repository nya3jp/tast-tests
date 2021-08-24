// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	apkPackageName = "org.chromium.arc.testapp.share"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NearbyShareSend,
		Desc: "Installs ARC share test app and share text/file to Nearby Share via Sharesheet",
		Contacts: []string{
			"alanding@chromium.org",
			"phshah@chromium.org",
			"arc-app-dev@google.com",
		},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcShareTestApp.apk"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
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

func NearbyShareSend(ctx context.Context, s *testing.State) {
	const (
		apkName = "ArcShareTestApp.apk"

		nearbyChipLabel    = "Nearby"
		nearbySharingTitle = "Nearby Share"

		// Class name of text view.
		textViewClassName = "android.widget.TextView"
		// Class name of check box.
		checkBoxClassName = "android.widget.CheckBox"
		// Class name of button.
		buttonClassName = "android.widget.Button"

		// Id of the check box for using text.
		checkBoxTextID = "checkBoxText"
		// Id of the check box for using the small sized file.
		checkBoxSmallFileID = "checkBoxSmallFile"
		// Id of the check box for using the medium sized file.
		checkBoxMediumFileID = "checkBoxMediumFile"
		// Id of the check box for using the large sized file.
		checkBoxLargeFileID = "checkBoxLargeFile"
		// Id of the button for sharing text or file.
		shareButtonID = "share_button"

		// Default UI timeout.
		uiTimeout = 7 * time.Second
	)

	s.Log("Connect to Chrome")

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.EnableFeatures("ArcNearbySharing"), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	s.Log("Installing APK")

	if err := a.Install(ctx, s.DataPath(apkName), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatal("Failed to install: ", err)
	}

	isVMEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to verify VM status: ", err)
	}
	if isVMEnabled {
		testing.ContextLog(ctx, "Granting permissions")

		// Android 11+ require granting MANAGE_EXTERNAL_STORAGE permission.
		if err := a.Command(ctx, "appops", "set", apkPackageName, "MANAGE_EXTERNAL_STORAGE", "allow").Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed granting all files access permission: ", err)
		}
	}

	testing.ContextLog(ctx, "Launching share test app")

	// Start app w/ fullscreen.
	if err := a.Command(ctx, "am", "start", "--windowingMode", "1", "-n", apkPackageName+"/.MainActivity").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Make sure the check box is available.
	testCheckBoxItem := d.Object(ui.ClassName(checkBoxClassName), shareUIID(checkBoxSmallFileID))
	if err := testCheckBoxItem.WaitForExists(ctx, uiTimeout); err != nil {
		s.Fatal("Failed to wait reading check box: ", err)
	}

	// Click the file sharing box.
	if err = shareUIClick(ctx, d, ui.ClassName(checkBoxClassName), shareUIID(checkBoxSmallFileID)); err != nil {
		s.Fatal("Failed to click text check box item: ", err)
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
		nearbyClass = textViewClassName
	}

	// Make sure the Nearby Share button is available.
	testNearbyItem := d.Object(ui.ClassName(nearbyClass), ui.TextMatches(nearbyText))
	if err := testNearbyItem.WaitForExists(ctx, uiTimeout); err != nil {
		s.Fatal("Failed to wait reading Nearby Share text view: ", err)
	}

	// Click to open the Nearby Share target.
	if err = shareUIClick(ctx, d, ui.ClassName(nearbyClass), ui.TextMatches(nearbyText)); err != nil {
		s.Fatal("Failed to select Nearby Share target: ", err)
	}
}
