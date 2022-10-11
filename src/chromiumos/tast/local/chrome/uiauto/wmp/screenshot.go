// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// TODO: Rename this file to be screen_capture_util.go.

// Package wmp contains utility functions for window management and performance.
package wmp

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

type screenshotType int
type screenRecordingType int

const (
	// FullScreenshot is the type to take a full screenshot.
	FullScreenshot screenshotType = iota
	// FullScreenRecording is the type to take a full screen recording.
	FullScreenRecording screenRecordingType = iota

	screenshotPattern      = "Screenshot*.png"
	screenRecordingPattern = "Screen recording*.webm"
)

// LaunchScreenCapture launches "Screen capture" from Quick Settings.
func LaunchScreenCapture(ctx context.Context, tconn *chrome.TestConn) error {
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to open Quick Settings")
	}
	defer func(ctx context.Context, tconn *chrome.TestConn) {
		if err := quicksettings.Hide(ctx, tconn); err != nil {
			testing.ContextLog(ctx, "Failed to hide Quick Settings: ", err)
		}
	}(ctx, tconn)

	return uiauto.New(tconn).LeftClick(quicksettings.PodIconButton(quicksettings.SettingPodScreenCapture))(ctx)
}

// CaptureScreenshot captures screenshot according to the argument passed in, fullscreen, partial or window.
func CaptureScreenshot(tconn *chrome.TestConn, sst screenshotType) action.Action {
	return func(ctx context.Context) error {
		if err := ensureInScreenCaptureMode(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to verify the ui of toolbar")
		}

		switch sst {
		case FullScreenshot:
			if err := takeFullScreenshot(tconn)(ctx); err != nil {
				return err
			}
		default:
			return errors.New("unknown screenshot type")
		}

		if err := screenshotTaken(tconn)(ctx); err != nil {
			return errors.Wrap(err, "failed to check the screenshot taken")
		}

		return nil
	}
}

// StartFullScreenRecording starts full screen recording by "Screen capture" in the quick settings.
func StartFullScreenRecording(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)

	return uiauto.Combine("take full screen recording",
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Screen record")),
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Record full screen")),
		ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile("(Click|Tap) anywhere to record full screen"))), // Different names for clamshell/tablet mode.
		ui.LeftClick(nodewith.Role(role.Window).First()),                                                         // Click on the center of root window to take the screenshot.
		ui.WaitUntilExists(nodewith.ClassName("TrayBackgroundView").Name("Stop screen recording")),
	)
}

// EndScreenRecording ends the in-progress screen recording.
func EndScreenRecording(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)

	return uiauto.Combine("end screen recording",
		ui.LeftClick(nodewith.ClassName("TrayBackgroundView").Name("Stop screen recording")),
		ui.WaitUntilExists(nodewith.ClassName("Label").Name("Screen recording taken")),
	)
}

// DeleteAllScreenCaptureFiles is an utility function to delete all screenshots and/or screen recordings.
func DeleteAllScreenCaptureFiles(downloadsPath string, deleteScreenshots, deleteScreenRecordings bool) error {
	if deleteScreenshots {
		files, err := filepath.Glob(filepath.Join(downloadsPath, screenshotPattern))
		if err != nil {
			return errors.Wrapf(err, "the pattern %q is malformed", screenshotPattern)
		}

		for _, f := range files {
			if err := os.Remove(f); err != nil {
				return errors.Wrap(err, "failed to delete the screenshots")
			}
		}
	}

	if deleteScreenRecordings {
		files, err := filepath.Glob(filepath.Join(downloadsPath, screenRecordingPattern))
		if err != nil {
			return errors.Wrapf(err, "the pattern %q is malformed", screenRecordingPattern)
		}

		for _, f := range files {
			if err := os.Remove(f); err != nil {
				return errors.Wrap(err, "failed to delete the screen recordings")
			}
		}
	}

	return nil
}

// CheckScreenshot checks the screenshot's existence.
// And then verifies its size is the same as the size of the full screen by decoding the screenshot.
func CheckScreenshot(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to obtain the display mode")
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the display orientation")
	}

	expectedFullScreenshotSize := coords.NewSize(displayMode.WidthInNativePixels, displayMode.HeightInNativePixels)
	// The screen ui orientation might be different from the DUT's default orientation setting.
	// If the orientation angle is 90 or 270 degrees,
	// swap the width and height value to match the current screen ui orientation.
	if orientation.Angle == 90 || orientation.Angle == 270 {
		expectedFullScreenshotSize.Width, expectedFullScreenshotSize.Height = expectedFullScreenshotSize.Height, expectedFullScreenshotSize.Width
	}

	files, err := filepath.Glob(filepath.Join(downloadsPath, screenshotPattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", screenshotPattern)
	}

	if len(files) == 0 {
		return errors.New("screenshot not found")
	} else if len(files) > 1 {
		return errors.Errorf("unexpected screeshot count, want 1, got %d", len(files))
	}

	// Expecting only one screenshot exist.
	imgFile := files[0]

	reader, err := os.Open(imgFile)
	if err != nil {
		return errors.Wrap(err, "failed to open the screenshot")
	}
	defer reader.Close()

	image, _, err := image.DecodeConfig(reader)
	if err != nil {
		return errors.Wrap(err, "failed to decode the screenshot")
	}

	if image.Width != expectedFullScreenshotSize.Width || image.Height != expectedFullScreenshotSize.Height {
		return errors.Errorf("screenshot size mismatched: want %s, got (%d x %d)", expectedFullScreenshotSize, image.Width, image.Height)
	}

	return nil
}

func ensureInScreenCaptureMode(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	// To make sure "Screen capture" is launched correctly, verify the existence of these buttons.
	for _, btn := range []*nodewith.Finder{
		nodewith.Role(role.ToggleButton).Name("Screenshot"),
		nodewith.Role(role.ToggleButton).Name("Screen record"),
		nodewith.Role(role.ToggleButton).NameRegex(regexp.MustCompile("(Take|Record) full screen.*")),
		nodewith.Role(role.ToggleButton).NameRegex(regexp.MustCompile("(Take|Record) partial screen.*")),
		nodewith.Role(role.ToggleButton).NameRegex(regexp.MustCompile("(Take|Record) window.*")),
		nodewith.Role(role.ToggleButton).Name("Settings"),
		nodewith.Role(role.Button).Name("Close").HasClass("CaptureModeButton"),
	} {
		if err := ui.WaitUntilExists(btn)(ctx); err != nil {
			return err
		}
	}

	return nil
}

// takeFullScreenshot takes full screenshot by "Screen capture" in the quick settings.
func takeFullScreenshot(tconn *chrome.TestConn) action.Action {
	ui := uiauto.New(tconn)

	return uiauto.Combine("take full screenshot",
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Screenshot")),
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Take full screen screenshot")),
		ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile("(Click|Tap) anywhere to capture full screen"))), // Different names for clamshell/tablet mode.
		ui.LeftClick(nodewith.Role(role.Window).First()),                                                          // Click on the center of root window to take the screenshot.
	)
}

// screenshotTaken checks the screenshot taken by the popup text "Screenshot taken".
func screenshotTaken(tconn *chrome.TestConn) action.Action {
	return uiauto.New(tconn).WaitUntilExists(nodewith.Role(role.StaticText).Name("Screenshot taken"))
}
