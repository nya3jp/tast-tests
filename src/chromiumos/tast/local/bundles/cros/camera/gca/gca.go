// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gca

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// GoogleCameraArc (GCA) package.
	pkg = "com.google.android.GoogleCameraArc"

	// ImageFormat is the filename format of images taken by GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
	ImageFormat = `IMG_\d{8}_\d{6}(?:_BURST)?(?:_COVER)?\.\w+`

	// VideoFormat is the filename format of videos taken by GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
	VideoFormat = `VID_\d{8}_\d{6}\.\w+`
)

// Mode refers to the capture mode GCA is in.
type Mode int

const (
	// PhotoMode refers to photo mode, in which users can take a photo.
	PhotoMode Mode = iota

	// VideoMode refers to video mode, in which users can record a video.
	VideoMode
)

// SwitchMode synchronously switches the current mode of GCA to the specified mode.
func SwitchMode(ctx context.Context, s *testing.State, d *ui.Device, mode Mode) {
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
		s.Fatal("GCA is already in the mode we want to switch to: ", err)
	} else {
		if err := switchButton.Click(ctx); err != nil {
			s.Fatal("Failed to click mode switch button: ", err)
		}
		if err := d.Object(ui.ID(shutterButtonID), ui.Description(shutterDescription)).WaitForExistsWithDefaultTimeout(ctx); err != nil {
			s.Fatal("Failed to switch mode: ", err)
		}
	}
}

// ClickShutterButton clicks the shutter button on the screen. This can be a regular photo shutter button or a recording button.
func ClickShutterButton(ctx context.Context, s *testing.State, d *ui.Device) {
	const shutterButtonID = "com.google.android.GoogleCameraArc:id/shutter_button"
	shutterButton := d.Object(ui.ID(shutterButtonID), ui.Clickable(true))
	if err := shutterButton.WaitForExistsWithDefaultTimeout(ctx); err != nil {
		s.Fatal("Failed to find shutter button: ", err)
	}
	if err := shutterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click shutter button: ", err)
	}
}

// VerifyFile examines the Downloads directory for any new files (modification time after specified timestamp) that match the specified pattern.
func VerifyFile(ctx context.Context, s *testing.State, pat string, ts time.Time) {
	// Get the Downloads directory where we save our media files.
	path, err := cryptohome.UserPath(s.PreValue().(arc.PreData).Chrome.User())
	if err != nil {
		s.Fatal("Failed to get user path: ", err)
	}
	path = filepath.Join(path, "Downloads")

	re := regexp.MustCompile(pat)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			s.Fatal("Failed to read the directory for saving media files: ", err)
		}
		for _, file := range files {
			if file.ModTime().After(ts) {
				s.Log("New file found: ", file.Name())
				if re.FindStringIndex(file.Name()) != nil {
					s.Log("Found a match: ", file.Name())
					return nil
				}
			}
		}
		return errors.Wrap(err, "failed to find output file")
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to find output file after 5 seconds: ", err)
	}
}

// SetUpDevice sets up the test environment, including starting UIAutomator server and launching GCA.
func SetUpDevice(ctx context.Context, s *testing.State) (*ui.Device, error) {
	const (
		// GCA Migration App. This app would would change the directory of media files to user's downloads folder and launch GCA after it's done.
		migrateIntent = "com.android.googlecameramigration/com.android.googlecameramigration.MainActivity"

		appRootViewID = "com.google.android.GoogleCameraArc:id/activity_root_view"
	)

	a := s.PreValue().(arc.PreData).ARC

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize UI Automator")
	}

	// GCA would ask for location permission during startup. We need to dismiss the dialog before we can use the app.
	s.Log("Granting all needed permissions (e.g., location) to GCA")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		return d, errors.Wrap(err, "failed to grant ACCESS_FINE_LOCATION permission to GCA")
	}
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		return d, errors.Wrap(err, "failed to grant ACCESS_COARSE_LOCATION permission to GCA")
	}

	// Starts the migration app to migrate media files and launch GCA.
	s.Log("Launching GCA Migration App (and GCA)")
	if err := a.Command(ctx, "am", "start", "-W", "-n", migrateIntent).Run(testexec.DumpLogOnError); err != nil {
		return d, errors.Wrap(err, "failed to start app")
	}

	if err := d.Object(ui.ID(appRootViewID)).WaitForExistsWithDefaultTimeout(ctx); err != nil {
		return d, errors.Wrap(err, "failed to load app")
	}

	return d, nil
}

// TearDownDevice tears down the test environment, including closing GCA and UIAutomator server.
func TearDownDevice(ctx context.Context, s *testing.State, d *ui.Device) {
	if d != nil {
		// Close GCA.
		a := s.PreValue().(arc.PreData).ARC
		if err := a.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to close GCA: ", err)
		}

		// Close UIAutomator server.
		d.Close()
	}
}
