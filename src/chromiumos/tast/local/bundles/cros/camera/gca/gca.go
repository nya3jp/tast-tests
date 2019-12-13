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
	// Apk is the name of the apk of GoogleCameraArc (GCA).
	Apk = "GoogleCameraArc.apk"
	// GoogleCameraArc (GCA) package.
	pkg             = "com.google.android.GoogleCameraArc"
	intent          = "com.android.camera.CameraLauncher"
	idBase          = pkg + ":id/"
	shutterButtonID = idBase + "shutter_button"

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
	// Back means the camera is back-facing.
	Back Facing = iota
	// Front means the camera is front-facing.
	Front
	// External means the camera is an external USB camera.
	External
)

// TimerOption refers to the setting of the countdown timer for capturing photos.
type TimerOption int

const (
	// NoTimer option disables the timer.
	NoTimer TimerOption = iota
	// ThreeSecondTimer sets the timer to 3 seconds.
	ThreeSecondTimer
	// TenSecondTimer sets the timer to 10 seconds.
	TenSecondTimer
)

func (facing Facing) String() string {
	switch facing {
	case Back:
		return "Back"
	case Front:
		return "Front"
	default:
		return "External"
	}
}

// GetFacing returns the direction the current camera is facing.
func GetFacing(ctx context.Context, d *ui.Device) (Facing, error) {
	const viewfinderID = idBase + "viewfinder_frame"
	viewfinder := d.Object(ui.ID(viewfinderID))
	if err := viewfinder.WaitForExists(ctx, shortTimeout); err != nil {
		return External, errors.Wrap(err, "failed to find viewfinder frame (did GCA crash?)")
	}
	var facing string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		camInfo, err := viewfinder.GetContentDescription(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get content description of viewfinder frame"))
		}
		s := strings.Split(camInfo, "|")
		if len(s) <= 1 {
			return errors.Errorf("failed to read camera info from viewfinder frame (read info %q)", camInfo)
		}
		facing = s[1]
		return nil
	}, &testing.PollOptions{Timeout: shortTimeout}); err != nil {
		return External, errors.Wrap(err, "timed out reading camera info")
	}
	switch facing {
	case "BACK":
		return Back, nil
	case "FRONT":
		return Front, nil
	case "EXTERNAL":
		return External, nil
	default:
		return External, errors.Errorf("unknown camera direction %q", facing)
	}
}

// SwitchMode synchronously switches the current mode of GCA to the specified mode.
func SwitchMode(ctx context.Context, d *ui.Device, mode Mode) error {
	var switchButtonID string
	switch mode {
	case PhotoMode:
		switchButtonID = idBase + "photo_switch_button"
	case VideoMode:
		switchButtonID = idBase + "video_switch_button"
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
	const switchButtonID = idBase + "camera_switch_button"
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

// SetTimerOption sets the countdown timer to the specified timer option.
func SetTimerOption(ctx context.Context, d *ui.Device, t TimerOption) error {
	const (
		timerButtonID            = idBase + "timer_button"
		noTimerButtonID          = idBase + "timer_off"
		threeSecondTimerButtonID = idBase + "timer_3s"
		tenSecondTimerButtonID   = idBase + "timer_10s"
	)
	timerButton := d.Object(ui.ID(timerButtonID), ui.Clickable(true))
	if err := timerButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "failed to find timer button")
	}
	// Expand timer option menu.
	if err := timerButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click timer button")
	}
	// Find and click the corresponding timer option.
	var timerOptionButtonID string
	switch t {
	case NoTimer:
		timerOptionButtonID = noTimerButtonID
	case ThreeSecondTimer:
		timerOptionButtonID = threeSecondTimerButtonID
	case TenSecondTimer:
		timerOptionButtonID = tenSecondTimerButtonID
	}
	timerOptionButton := d.Object(ui.ID(timerOptionButtonID), ui.Clickable(true))
	if err := timerOptionButton.WaitForExists(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "failed to find the specified timer option")
	}
	if err := timerOptionButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click timer option")
	}
	// Wait until the timer option is set.
	if err := d.WaitForIdle(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "failed to select specified timer option")
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
	path, err := cryptohome.UserPath(ctx, cr.User())
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
func setUpDevice(ctx context.Context, a *arc.ARC, apkPath string) (*ui.Device, error) {
	var permissions = []string{"ACCESS_FINE_LOCATION", "ACCESS_COARSE_LOCATION", "CAMERA", "RECORD_AUDIO", "WRITE_EXTERNAL_STORAGE"}

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

	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, apkPath); err != nil {
		return nil, errors.Wrap(err, "failed to install app")
	}

	// GCA would ask for location permission during startup. We need to dismiss the dialog before we can use the app.
	testing.ContextLog(ctx, "Granting all needed permissions (e.g., location) to GCA")
	for _, permission := range permissions {
		if err := a.Command(ctx, "pm", "grant", pkg, "android.permission."+permission).Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrapf(err, "failed to grant %s permission to GCA", permission)
		}
	}

	// Launch GCA.
	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/"+intent).Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to start GCA")
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
	d, err := setUpDevice(ctx, a, s.DataPath(Apk))
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
