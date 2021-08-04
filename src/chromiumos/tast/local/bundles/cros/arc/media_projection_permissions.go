// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// mediaProjAPK is the name of the APK to be installed for this test.
// mediaProjPkg is the package name used in the above APK.
// mediaProjActivity is the main activity used in the above APK.
const (
	mediaProjAPK      = "ArcMediaProjectionPermissionsTest.apk"
	mediaProjPkg      = "org.chromium.arc.testapp.mediaprojection"
	mediaProjActivity = "PermissionsActivity"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaProjectionPermissions,
		Desc:         "Checks that Chrome permissions dialog is used when using the MediaProjection API",
		Contacts:     []string{"cherieccy@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		SoftwareDeps: []string{"chrome", "android_p"},
		Fixture:      "arcBooted",
		Data:         []string{mediaProjAPK},
	})
}

// When using the MediaProjection API in ARC++/ARCVM, we use the Desktop Media
// Picker in Chrome, instead of the Android permissions dialog, to handle
// permissions. A notification in the system tray is also shown during screen
// capture. This tast test ensures that we are using the Chrome dialog and
// showing the notification. It does not cover screen capture testing because
// that is covered by CTS.

func MediaProjectionPermissions(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := setupMediaProjection(ctx, tconn, a, s.DataPath(mediaProjAPK)); err != nil {
		s.Fatal("Failed to set up media projection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Open media projection dialog, but cancel it immediately.
	if err := openMediaProjectionDialog(ctx, tconn, d, "Cancel", "cancelled"); err != nil {
		s.Fatal("Failed to open media projection dialog and cancel: ", err)
	}

	// Open media projection dialog and start sharing.
	if err := openMediaProjectionDialog(ctx, tconn, d, "Share", "started"); err != nil {
		s.Fatal("Failed to open media projection dialog and share: ", err)
	}

	// Look for a notification.
	if err := checkMediaProjectionNotification(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for media projection notification: ", err)
	}

	testing.ContextLog(ctx, "Stop media projection")
	stopButton := d.Object(androidui.ID(mediaProjPkg + ":id/stop_button"))
	if err := stopButton.Click(ctx); err != nil {
		s.Fatal("Unable to click the stop button: ", err)
	}

	if err := checkMediaProjectionStatus(ctx, d, "stopped"); err != nil {
		s.Fatal("Failed to check media projection status: ", err)
	}

	// No more notification after sharing is stopped.
	if err := checkMediaProjectionNotification(ctx, tconn); err == nil {
		s.Fatal("Media projection notification is expected to be gone, but it is still shown")
	}
}

// setupMediaProjection installs ArcMediaProjectionPermissionsTest.apk and
// start its main activity.
func setupMediaProjection(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, apkPath string) error {
	if err := a.Install(ctx, apkPath); err != nil {
		return errors.Wrap(err, "failed to install the app: ")
	}

	act, err := arc.NewActivity(a, mediaProjPkg, "."+mediaProjActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create the activity: ")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start the activity: ")
	}

	return nil
}

// openMediaProjectionDialog starts media projection, waits for the media
// projection dialog, clicks an action button (Cancel / Share) and verifies
// the media projection status.
func openMediaProjectionDialog(ctx context.Context, tconn *chrome.TestConn, d *androidui.Device, actionButtonName, expectedStatus string) error {
	testing.ContextLog(ctx, "Start media projection")
	startButton := d.Object(androidui.ID(mediaProjPkg + ":id/start_button"))
	if err := startButton.Click(ctx); err != nil {
		return errors.Wrap(err, "unable to click start button: ")
	}

	testing.ContextLog(ctx, "Open media projection dialog and click "+actionButtonName)
	dialog := nodewith.ClassName("DesktopMediaPickerDialogView").Name("Share your entire screen").Role(role.Window)
	shareTarget := nodewith.ClassName("DesktopMediaSourceView").Name("Built-in display").Role(role.Button)
	button := nodewith.ClassName("MdTextButton").Name(actionButtonName).Role(role.Button)
	ui := uiauto.New(tconn)
	if err := uiauto.Combine("Open media projection dialog and click "+actionButtonName,
		ui.WaitUntilExists(dialog),
		ui.LeftClick(shareTarget),
		ui.LeftClick(button),
		ui.WaitUntilGone(dialog),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open media projection dialog: ")
	}

	if err := checkMediaProjectionStatus(ctx, d, expectedStatus); err != nil {
		return errors.Wrap(err, "failed to check media projection status: ")
	}

	return nil
}

// checkMediaProjectionStatus verifies the media projection status shown in the
// app against the expected status.
func checkMediaProjectionStatus(ctx context.Context, d *androidui.Device, expectedStatus string) error {
	statusText := d.Object(androidui.ID(mediaProjPkg + ":id/status_text"))
	text, err := statusText.GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get status text: ")
	}
	if text != expectedStatus {
		return errors.Errorf("Wrong media projection status: got %s; want %s", text, expectedStatus)
	}
	return nil
}

// checkMediaProjectionNotification looks for media projection notification
// with ID "chrome://screen/capture".
func checkMediaProjectionNotification(ctx context.Context, tconn *chrome.TestConn) error {
	const timeout = 5 * time.Second
	testing.ContextLog(ctx, "Check for media projection notification")
	_, err := ash.WaitForNotification(ctx, tconn, timeout, ash.WaitIDContains("chrome://screen/capture"),
		ash.WaitTitle("You're sharing your screen"))
	return err
}
