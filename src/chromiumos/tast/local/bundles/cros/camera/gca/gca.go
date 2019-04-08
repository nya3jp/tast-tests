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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// GoogleCameraArc (GCA) package.
	pkg = "com.google.android.GoogleCameraArc"

	shortTimeout = 5 * time.Second
	longTimeout  = 10 * time.Second
)

var (
	// ImagePattern is the filename format of images taken by GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
	ImagePattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}(?:_BURST)?(?:_COVER)?\.\w+$`)

	// VideoPattern is the filename format of videos taken by GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
	VideoPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}\.\w+$`)
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
func SwitchMode(ctx context.Context, d *ui.Device, mode Mode) error {
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
	if err := switchButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "already in the mode the test wants to switch to")
	}
	if err := switchButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click mode switch button")
	}
	if err := d.Object(ui.ID(shutterButtonID), ui.Description(shutterDescription)).WaitForExists(ctx, longTimeout); err != nil {
		return errors.Wrap(err, "failed to switch mode")
	}
	return nil
}

// ClickShutterButton clicks the shutter button on the screen. This can be a regular photo shutter button or a recording button.
func ClickShutterButton(ctx context.Context, d *ui.Device) error {
	const shutterButtonID = "com.google.android.GoogleCameraArc:id/shutter_button"
	shutterButton := d.Object(ui.ID(shutterButtonID), ui.Clickable(true))
	if err := shutterButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "failed to find shutter button")
	}
	if err := shutterButton.Click(ctx); err != nil {
		return errors.Wrap(err, "Failed to click shutter button")
	}
	return nil
}

// VerifyFile examines the Downloads directory for any new files (modification time after specified timestamp) that match the specified pattern.
func VerifyFile(ctx context.Context, cr *chrome.Chrome, pat *regexp.Regexp, ts time.Time) error {
	// Get the Downloads directory where we save our media files.
	path, err := cryptohome.UserPath(cr.User())
	if err != nil {
		return errors.Wrap(err, "failed to get user path")
	}
	path = filepath.Join(path, "Downloads")

	testing.ContextLog(ctx, "Looking for output file")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			return errors.Wrap(err, "failed to read the directory for saving media files")
		}
		for _, file := range files {
			if file.ModTime().Before(ts) {
				continue
			}
			testing.ContextLog(ctx, "New file found: ", file.Name())
			if pat.MatchString(file.Name()) {
				testing.ContextLog(ctx, "Found a match: ", file.Name())
				return nil
			}
		}
		return errors.New("no matching output file found")
	}, &testing.PollOptions{Timeout: shortTimeout}); err != nil {
		return errors.Wrapf(err, "no matching output file found after %f seconds: %s", shortTimeout.Seconds())
	}
	return nil
}

// SetUpDevice sets up the test environment, including starting UIAutomator server and launching GCA.
func SetUpDevice(ctx context.Context, a *arc.ARC) (*ui.Device, error) {
	const (
		// GCA Migration App. This app would change the directory of media files to user's downloads folder and launch GCA after it's done.
		migrateIntent = "com.android.googlecameramigration/com.android.googlecameramigration.MainActivity"

		appRootViewID = "com.google.android.GoogleCameraArc:id/activity_root_view"
	)
	success := false
	var d *ui.Device
	defer func() {
		if !success {
			TearDownDevice(ctx, a, d)
		}
	}()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize UIAutomator")
	}

	// GCA would ask for location permission during startup. We need to dismiss the dialog before we can use the app.
	testing.ContextLog(ctx, "Granting all needed permissions (e.g., location) to GCA")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		return d, errors.Wrap(err, "failed to grant ACCESS_FINE_LOCATION permission to GCA")
	}
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		return d, errors.Wrap(err, "failed to grant ACCESS_COARSE_LOCATION permission to GCA")
	}

	// Starts the migration app to migrate media files and launch GCA.
	testing.ContextLog(ctx, "Launching GCA Migration App (and GCA)")
	if err := a.Command(ctx, "am", "start", "-W", "-n", migrateIntent).Run(testexec.DumpLogOnError); err != nil {
		return d, errors.Wrap(err, "failed to start app")
	}

	if err := d.Object(ui.ID(appRootViewID)).WaitForExists(ctx, longTimeout); err != nil {
		return d, errors.Wrap(err, "failed to load app")
	}

	success = true
	return d, nil
}

// TearDownDevice tears down the test environment, including closing GCA and UIAutomator server.
func TearDownDevice(ctx context.Context, a *arc.ARC, d *ui.Device) error {
	var firstErr error
	if d != nil {
		// Close GCA.
		if err := a.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			err = errors.Wrap(err, "failed to close GCA")
			testing.ContextLog(ctx, "Error during teardown: ", err)
			if firstErr == nil {
				firstErr = err
			}
		}

		// Close UIAutomator server.
		/*
		   if err := d.Close(); err != nil {
		     err = errors.Wrap(err, "failed to close UIAutomator server")
		     testing.ContextLog(ctx, "Error during teardown: ", err)
		     if firstErr == nil {
		       firstErr = err
		     }
		   }*/
		d.Close()
	}
	return firstErr
}
