// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gca

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
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
	pkg             = "com.google.android.GoogleCameraArc"
	shutterButtonID = "com.google.android.GoogleCameraArc:id/shutter_button"

	// Predefined timeouts for ease of scaling.
	// shortTimeout is a shorter timeout for operations that should complete quickly.
	shortTimeout = 5 * time.Second
	// longTimeout is a longer timeout for operations that takes longer.
	longTimeout = 10 * time.Second
)

var (
	// ImagePattern is the filename format of images taken by GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
	ImagePattern = regexp.MustCompile(`^IMG_\d{8}_\d{6}(?:_BURST)?(?:_COVER)?\.\w+$`)

	// VideoPattern is the filename format of videos taken by GCA.
	// See: google3/java/com/google/android/apps/chromeos/camera/storage/filenamer/FileNamerModule.java
	VideoPattern = regexp.MustCompile(`^VID_\d{8}_\d{6}\.\w+$`)
)

// TestFunc is the body of a test and is called after the test environment is setup.
type TestFunc func(context.Context, *arc.ARC, *ui.Device)

// Mode refers to the capture mode GCA is in.
type Mode int

const (
	// PhotoMode refers to photo mode, in which users can take a photo.
	PhotoMode Mode = iota

	// VideoMode refers to video mode, in which users can record a video.
	VideoMode
)

// Facing refers to the direction a camera is facing.
type Facing int

const (
	// Back refers to a back-facing camera.
	Back Facing = iota

	// Front refers to a front-facing camera.
	Front

	// External refers to an external camera.
	External

	// Unknown indicates a camera with unknown facing. This should never be the case unless an error occurred while retrieving facing information.
	Unknown
)

func (facing Facing) String() string {
	switch facing {
	case Back:
		return "Back"
	case Front:
		return "Front"
	case External:
		return "External"
	default:
		return "Unknown"
	}
}

// GetCameraFacing returns the direction the current camera is facing.
func GetCameraFacing(ctx context.Context, d *ui.Device) (Facing, error) {
	const viewfinderID = "com.google.android.GoogleCameraArc:id/viewfinder_frame"
	viewfinder := d.Object(ui.ID(viewfinderID))
	if err := viewfinder.WaitForExists(ctx, shortTimeout); err != nil {
		return Unknown, errors.Wrap(err, "failed to find viewfinder frame (did GCA crash?)")
	}
	camInfo, err := viewfinder.GetContentDescription(ctx)
	if err != nil {
		return Unknown, errors.Wrap(err, "failed to get content description of viewfinder frame")
	}
	s := strings.Split(camInfo, "|")
	if len(s) <= 1 {
		return Unknown, errors.New("failed to read camera info from viewfinder frame")
	}
	switch facing := s[1]; facing {
	case "BACK":
		return Back, nil
	case "FRONT":
		return Front, nil
	case "EXTERNAL":
		return External, nil
	default:
		return Unknown, errors.Errorf("camera direction is unknown: ", facing)
	}
}

// SwitchMode synchronously switches the current mode of GCA to the specified mode.
func SwitchMode(ctx context.Context, d *ui.Device, mode Mode) error {
	var switchButtonID string
	switch mode {
	case PhotoMode:
		switchButtonID = "com.google.android.GoogleCameraArc:id/photo_switch_button"
	case VideoMode:
		switchButtonID = "com.google.android.GoogleCameraArc:id/video_switch_button"
	}
	switchButton := d.Object(ui.ID(switchButtonID), ui.Clickable(true))
	if err := switchButton.WaitForExists(ctx, shortTimeout); err != nil {
		testing.ContextLog(ctx, "GCA is already in the mode the test wants to switch to")
		return nil
	}
	if err := switchButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click mode switch button")
	}
	// Wait until the switch button is gone (the other switch button would appear). This indicates the mode switch is done.
	if err := d.Object(ui.ID(switchButtonID)).WaitUntilGone(ctx, longTimeout); err != nil {
		return errors.Wrap(err, "failed to switch mode")
	}
	return nil
}

// SwitchCamera synchronously switches the app to the next camera.
func SwitchCamera(ctx context.Context, d *ui.Device) error {
	const switchButtonID = "com.google.android.GoogleCameraArc:id/camera_switch_button"
	switchButton := d.Object(ui.ID(switchButtonID), ui.Clickable(true))
	if err := switchButton.WaitForExists(ctx, shortTimeout); err != nil {
		testing.ContextLog(ctx, "Failed to find camera switch button")
		// We might not have a camera switch button if the device only has one camera.
		return nil
	}
	if err := switchButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click camera switch button")
	}
	// Wait until buttons are clickable again (buttons won't be clickable until preview has successfully started)
	if err := switchButton.WaitForExists(ctx, longTimeout); err != nil {
		return errors.Wrap(err, "preview failed to start")
	}
	return nil
}

// ClickShutterButton clicks the shutter button on the screen. This can be a regular photo shutter button or a recording button.
func ClickShutterButton(ctx context.Context, d *ui.Device) error {
	shutterButton := d.Object(ui.ID(shutterButtonID), ui.Clickable(true))
	if err := shutterButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "failed to find shutter button")
	}
	// Click the shutter button.
	if err := shutterButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click shutter button")
	}
	// Wait until the shutter button is clickable again. This indicates that the previous capture is done.
	if err := shutterButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "capture couldn't be completed successfully")
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
		return errors.Wrapf(err, "no matching output file found after %v", shortTimeout)
	}
	return nil
}

// RestartApp restarts GCA.
func RestartApp(ctx context.Context, a *arc.ARC, d *ui.Device) error {
	const intent = "com.android.camera.CameraLauncher"

	if err := a.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to stop GCA")
	}

	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/"+intent).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to start GCA")
	}

	if err := d.WaitForIdle(ctx, longTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for app to become idle while loading app")
	}
	return nil
}

// setUpDevice sets up the test environment, including starting UIAutomator server and launching GCA.
func setUpDevice(ctx context.Context, a *arc.ARC) (*ui.Device, error) {
	const (
		// GCA Migration App. This app would change the directory of media files to user's downloads folder and launch GCA after it's done.
		migrateIntent = "com.android.googlecameramigration/com.android.googlecameramigration.MainActivity"

		appRootViewID = "com.google.android.GoogleCameraArc:id/activity_root_view"
	)
	success := false
	var d *ui.Device
	defer func() {
		if !success {
			tearDownDevice(ctx, a, d)
		}
	}()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize UIAutomator")
	}

	// GCA would ask for location permission during startup. We need to dismiss the dialog before we can use the app.
	testing.ContextLog(ctx, "Granting all needed permissions (e.g., location) to GCA")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to grant ACCESS_FINE_LOCATION permission to GCA")
	}
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to grant ACCESS_COARSE_LOCATION permission to GCA")
	}

	// Starts the migration app to migrate media files and launch GCA.
	testing.ContextLog(ctx, "Launching GCA Migration App (and GCA)")
	if err := a.Command(ctx, "am", "start", "-W", "-n", migrateIntent).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to start app")
	}

	if err := d.WaitForIdle(ctx, longTimeout); err != nil {
		return nil, errors.Wrap(err, "failed to wait for app to become idle while loading app")
	}

	success = true
	return d, nil
}

// tearDownDevice tears down the test environment, including closing GCA and UIAutomator server.
func tearDownDevice(ctx context.Context, a *arc.ARC, d *ui.Device) error {
	if d == nil {
		// Nothing to shutdown since UIAutomator server isn't even up.
		return nil
	}
	var firstErr error
	// Close GCA.
	if err := a.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
		err = errors.Wrap(err, "failed to close GCA")
		testing.ContextLog(ctx, "Error during teardown: ", err)
		if firstErr == nil {
			firstErr = err
		}
	}

	// Close UIAutomator server.
	// TODO(lnishan): Check the error. Currently Close() always returns an error.
	d.Close()
	return firstErr
}

// RunTest sets up the device, runs the specified test and tears down the device afterwards.
func RunTest(ctx context.Context, s *testing.State, f TestFunc) {
	// Setup device.
	a := s.PreValue().(arc.PreData).ARC
	d, err := setUpDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to set up device: ", err)
	}
	defer func() {
		if err := tearDownDevice(ctx, a, d); err != nil {
			s.Error("Failed to tear down device: ", err)
		}
	}()

	// Shorten the context to save time for the teardown procedures.
	shortCtx, cancel := ctxutil.Shorten(ctx, longTimeout)
	defer cancel()

	// Run the test.
	f(shortCtx, a, d)
}
