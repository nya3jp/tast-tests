// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gca

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type gcaMode int

const (
	// PhotoMode refers to photo mode, in which users can take a photo.
	PhotoMode gcaMode = iota

	// VideoMode refers to video mode, in which users can record a video.
	VideoMode
)

// TestFunc is the body of test, run after the test environment (e.g., ARC++, UIAutomator) is setup.
type TestFunc func(context.Context, *testing.State, *ui.Device)

// SwitchMode synchronously switches the current mode of GCA to the specified mode.
func SwitchMode(ctx context.Context, s *testing.State, d *ui.Device, mode gcaMode) {
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
	switchButton := d.Object(ui.ID(switchButtonID))
	if err := switchButton.WaitForExistsWithDefaultTimeout(ctx); err != nil {
		s.Log("Failed to find mode switch button (maybe GCA is already in this mode?): ", err)
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
	shutterButton := d.Object(ui.ID(shutterButtonID))
	if err := shutterButton.WaitForExistsWithDefaultTimeout(ctx); err != nil {
		s.Fatal("Failed to find shutter button: ", err)
	}
	if err := shutterButton.Click(ctx); err != nil {
		s.Fatal("Failed to click shutter button: ", err)
	}
}

// RunTest setups the test environment (brings up ARC++, UIAutomator ...etc.) and runs the specified test function.
func RunTest(ctx context.Context, s *testing.State, f TestFunc) {
	const (
		pkg    = "com.google.android.GoogleCameraArc"
		intent = "com.android.camera.CameraLauncher"

		appRootViewID = "com.google.android.GoogleCameraArc:id/activity_root_view"
	)

	// TODO(lnishan): We might still need Chrome for cryptohome to access Downloads folder and verify a file has been saved.
	a := s.PreValue().(arc.PreData).ARC

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	// GCA would ask for location permission during startup. We need to dismiss
	// the dialog before we can use the app.
	s.Log("Granting all needed permissions (e.g., location) to GCA")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant ACCESS_FINE_LOCATION permission to GCA")
	}
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant ACCESS_COARSE_LOCATION permission to GCA")
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/"+intent).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	if err := d.Object(ui.ID(appRootViewID)).WaitForExistsWithDefaultTimeout(ctx); err != nil {
		s.Fatal("Failed to load app: ", err)
	}

	f(ctx, s, d)
}
