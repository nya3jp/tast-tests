// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gca

import (
	"context"
	"io/ioutil"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ImageFormat is the filename format of images taken by GCA.
// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
const ImageFormat = `IMG_\d{8}_\d{6}(?:_BURST)?(?:_COVER)?\.\w+`

// VideoFormat is the filename format of videos taken by GCA.
// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
const VideoFormat = `VID_\d{8}_\d{6}\.\w+`

// Mode refers to the capture mode GCA is in.
type Mode int

const (
	// PhotoMode refers to photo mode, in which users can take a photo.
	PhotoMode Mode = iota

	// VideoMode refers to video mode, in which users can record a video.
	VideoMode
)

// TestFunc is the body of test, run after the test environment (e.g., ARC++, UIAutomator) is setup.
type TestFunc func(context.Context, *testing.State, *ui.Device)

// SwitchMode synchronously switches the current mode of GCA to the specified mode.
func SwitchMode(ctx context.Context, s *testing.State, d *ui.Device, mode Mode) error {
	const shutterButtonID = "com.google.android.GoogleCameraArc:id/shutter_button"
	// shutterDescription should be updated with the latest source of GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/shutterbutton/res/values/strings.xml
	var switchButtonID, shutterDescription string
	switch mode {
	case PhotoMode:
		switchButtonID = "com.google.android.GoogleCameraArc:id/photo_switch_button"
		shutterDescription = "Shutter"
	case VideoMode:
		switchButtonID = "com.google.android.GoogleCameraArc:id/video_switch_button"
		shutterDescription = "Start Recording"
	}
	switchButton := d.Object(ui.ID(switchButtonID), ui.Clickable(true))
	if err := switchButton.WaitForExistsWithDefaultTimeout(ctx); err != nil {
		s.Log("Failed to find mode switch button (maybe GCA is already in this mode?): ", err)
	} else {
		if err := switchButton.Click(ctx); err != nil {
			return errors.Wrap(err, "Failed to click mode switch button")
		}
		if err := d.Object(ui.ID(shutterButtonID), ui.Description(shutterDescription)).WaitForExistsWithDefaultTimeout(ctx); err != nil {
			return errors.Wrap(err, "Failed to switch mode")
		}
	}
	return nil
}

// ClickShutterButtonAndVerifyFile clicks the shutter button and verifies whether there is an output file that matches the specified pattern.
func ClickShutterButtonAndVerifyFile(ctx context.Context, s *testing.State, d *ui.Device, pat string) error {
	// Get the Downloads directory where we save our media files.
	path, err := cryptohome.UserPath(s.PreValue().(arc.PreData).Chrome.User())
	path += "/Downloads"

	// Get the list of files before capturing.
	prevFiles, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrap(err, "Failed to read the directory for saving media files")
	}
	seen := make(map[string]bool)
	for _, file := range prevFiles {
		seen[file.Name()] = true
	}

	// Click shutter button to take a picture or record a video.
	s.Log("Clicking shutter button to take a picture of record a video")
	ClickShutterButton(ctx, s, d)

	// Check if any new files match the expected pattern.
	s.Log("Searching if any new files in the directory match the specified pattern")
	re := regexp.MustCompile(pat)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		curFiles, err := ioutil.ReadDir(path)
		if err != nil {
			return errors.Wrap(err, "Failed to read the directory for saving media files")
		}
		for _, file := range curFiles {
			if !seen[file.Name()] {
				s.Log("New file found: ", file.Name())
				if re.FindStringIndex(file.Name()) != nil {
					s.Log("Found a match: ", file.Name())
					return nil
				}
			}
		}
		return errors.New("Failed to find output file")
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "Failed to find output file after 5 seconds")
	}

	return nil
}

// ClickShutterButton clicks the shutter button on the screen. This can be a regular photo shutter button or a recording button.
func ClickShutterButton(ctx context.Context, s *testing.State, d *ui.Device) error {
	const shutterButtonID = "com.google.android.GoogleCameraArc:id/shutter_button"
	shutterButton := d.Object(ui.ID(shutterButtonID), ui.Clickable(true))
	if err := shutterButton.WaitForExistsWithDefaultTimeout(ctx); err != nil {
		return errors.Wrap(err, "Failed to find shutter button")
	}
	if err := shutterButton.Click(ctx); err != nil {
		return errors.Wrap(err, "Failed to click shutter button")
	}
	return nil
}

// RunTest setups the test environment (brings up ARC++, UIAutomator ...etc.) and runs the specified test function.
func RunTest(ctx context.Context, s *testing.State, f TestFunc) {
	const (
		// GoogleCameraArc (GCA) package.
		pkg    = "com.google.android.GoogleCameraArc"
		intent = "com.android.camera.CameraLauncher"

		// GCA Migration App. This app would would change the directory of media files to user's downloads folder and launch GCA after it's done.
		migratePkg = "com.android.googlecameramigration"
		migrateCls = "com.android.googlecameramigration.MainActivity"

		appRootViewID = "com.google.android.GoogleCameraArc:id/activity_root_view"
	)

	a := s.PreValue().(arc.PreData).ARC

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	// GCA would ask for location permission during startup. We need to dismiss the dialog before we can use the app.
	s.Log("Granting all needed permissions (e.g., location) to GCA")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant ACCESS_FINE_LOCATION permission to GCA")
	}
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant ACCESS_COARSE_LOCATION permission to GCA")
	}

	// Starts the migration app to migrate media files and launch GCA.
	s.Log("Launching GCA Migration App (and GCA)")
	if err := a.Command(ctx, "am", "start", "-W", "-n", migratePkg+"/"+migrateCls).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	if err := d.Object(ui.ID(appRootViewID)).WaitForExistsWithDefaultTimeout(ctx); err != nil {
		s.Fatal("Failed to load app: ", err)
	}

	f(ctx, s, d)
}
