// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmp contains utility functions for window management and performance.
package wmp

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// CaptureModeSource refers to the three capture mode sources in screen
// capture mode.
type CaptureModeSource int

// List out three capture mode sources.
const (
	FullScreen CaptureModeSource = iota
	PartialScreen
	Window
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

// CaptureScreenshot captures screenshot based on the argument passed in i.e. fullscreen, partial or window.
func CaptureScreenshot(ctx context.Context, tconn *chrome.TestConn, source CaptureModeSource) action.Action {
	return func(ctx context.Context) error {
		if err := ensureInScreenCaptureMode(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to verify the ui of toolbar")
		}

		switch source {
		case FullScreen:
			if err := takeFullScreenshot(ctx, tconn); err != nil {
				return err
			}
		case PartialScreen:
			if err := takePartialScreenshot(ctx, tconn); err != nil {
				return err
			}
		case Window:
			if err := takeWindowScreenshot(ctx, tconn); err != nil {
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
func takeFullScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	if err := uiauto.Combine("take full screenshot",
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Screenshot")),
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Take full screen screenshot")),
		ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile("(Click|Tap) anywhere to capture full screen"))), // Different names for clamshell/tablet mode.
		ui.LeftClick(nodewith.Role(role.Window).First()),                                                          // Click on the center of root window to take the screenshot.
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to take fullscreen screenshot")
	}
	return nil
}

// takePartialScreenshot selects a partial region and performs partial screenshot capture.
func takePartialScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	screens, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}
	captureSurfaceBounds := screens[0].WorkArea
	captureSurfaceCenterPt := captureSurfaceBounds.CenterPoint()

	if err = uiauto.Combine("take partial screenshot in small region",
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Screenshot")),
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Take partial screenshot")),
		ui.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile("Drag to select an area to capture"))), // Different names for clamshell/tablet mode.
		mouse.Move(tconn, coords.NewPoint(10, 10), 0),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, coords.NewPoint(15, 15), 0),
		mouse.Release(tconn, mouse.LeftButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to create the partial capture region")
	}
	captureButton := nodewith.Role(role.Button).Name("Capture")
	captureButtonLoc, err := ui.Location(ctx, captureButton)
	if err != nil {
		return errors.Wrap(err, "failed to get the location of capture button")
	}
	if captureButtonLoc.Left < 1 || captureButtonLoc.Top < 1 {
		return errors.Wrap(err, "failed to show the capture button outside the region when region becomes too small")
	}

	if err = uiauto.Combine("take partial screenshot",
		mouse.Move(tconn, coords.NewPoint(0, 0), 0),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, coords.NewPoint(captureSurfaceCenterPt.X/2, captureSurfaceCenterPt.Y/2), time.Second),
		mouse.Release(tconn, mouse.LeftButton),
		mouse.Move(tconn, coords.NewPoint(captureSurfaceCenterPt.X/8, captureSurfaceCenterPt.Y/8), time.Second),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, coords.NewPoint(captureSurfaceCenterPt.X/4, captureSurfaceCenterPt.Y/4), time.Second),
		mouse.Release(tconn, mouse.LeftButton),
		// Click outside the region create a new region.
		mouse.Move(tconn, coords.NewPoint(0, 0), 0),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, coords.NewPoint(captureSurfaceCenterPt.X, captureSurfaceCenterPt.Y), time.Second),
		mouse.Release(tconn, mouse.LeftButton),
		ui.LeftClick(nodewith.Role(role.Button).Name("Capture")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to take partial screenshot")
	}
	return nil
}

// takeWindowScreenshot opens a window and performs window screenshot capture.
func takeWindowScreenshot(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch the app")
	}

	activeWindow, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find active window")
	}
	centerPoint := activeWindow.BoundsInRoot.CenterPoint()

	if err = uiauto.Combine("take window screenshot",
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Screenshot")),
		ui.LeftClick(nodewith.Role(role.ToggleButton).Name("Take window screenshot")),
		mouse.Move(tconn, centerPoint, time.Second),      // Different names for clamshell/tablet mode.
		ui.LeftClick(nodewith.Role(role.Window).First()), // Click on the center of root window to take the screenshot.
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to take window screenshot")
	}

	return nil
}

// screenshotTaken checks the screenshot taken by the popup text "Screenshot taken".
func screenshotTaken(tconn *chrome.TestConn) action.Action {
	return uiauto.New(tconn).WaitUntilExists(nodewith.Role(role.StaticText).Name("Screenshot taken"))
}

// CheckScreenshot checks screenshot based on the argument passed in i.e. fullscreen, partial or window. The existence and the size of the
// captured image will be verified.
func CheckScreenshot(ctx context.Context, tconn *chrome.TestConn, downloadsPath string, source CaptureModeSource) error {
	switch source {
	case FullScreen:
		if err := checkFullScreenScreenshot(ctx, tconn, downloadsPath); err != nil {
			return err
		}
	case PartialScreen:
		if err := checkPartialScreenshot(ctx, tconn, downloadsPath); err != nil {
			return err
		}
	case Window:
		if err := checkWindowScreenshot(ctx, tconn, downloadsPath); err != nil {
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

func checkFullScreenScreenshot(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	imageConfig, err := getCaptureImageConfig(downloadsPath)
	if err != nil {
		return errors.Wrap(err, "failed to get image config")
	}
	expectedFullScreenshotSize, err := getCaptureSurfaceFullScreenBounds(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get full screen bounds")
	}

	if imageConfig.Width != expectedFullScreenshotSize.Width || imageConfig.Height != expectedFullScreenshotSize.Height {
		return errors.Errorf("full screen screenshot size mismatched: want %s, got (%d x %d)", expectedFullScreenshotSize, imageConfig.Width, imageConfig.Height)
	}

	return nil
}

func checkPartialScreenshot(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	imageConfig, err := getCaptureImageConfig(downloadsPath)
	if err != nil {
		return errors.Wrap(err, "failed to get image config")
	}

	screens, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get display info")
	}
	expectedPartialScreenshotSize := coords.NewSize(screens[0].WorkArea.Width/2, screens[0].WorkArea.Height/2)

	if imageConfig.Width != expectedPartialScreenshotSize.Width || imageConfig.Height != expectedPartialScreenshotSize.Height {
		return errors.Errorf("partial screenshot size mismatched: want %s, got (%d x %d)", expectedPartialScreenshotSize, imageConfig.Width, imageConfig.Height)
	}

	return nil
}

func checkWindowScreenshot(ctx context.Context, tconn *chrome.TestConn, downloadsPath string) error {
	imageConfig, err := getCaptureImageConfig(downloadsPath)
	if err != nil {
		return errors.Wrap(err, "failed to get image config")
	}

	activeWindow, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find active window")
	}
	windowBounds := activeWindow.BoundsInRoot

	if imageConfig.Width != windowBounds.Width || imageConfig.Height != windowBounds.Height {
		return errors.Errorf("window screenshot size mismatched: want %s, got (%d x %d)", windowBounds, imageConfig.Width, imageConfig.Height)
	}

	return nil
}

func getCaptureImageConfig(downloadsPath string) (image.Config, error) {
	const screenshotPattern = "Screenshot*.png"
	files, err := filepath.Glob(filepath.Join(downloadsPath, screenshotPattern))
	if err != nil {
		return image.Config{}, err
	}

	if len(files) == 0 {
		return image.Config{}, err
	} else if len(files) > 1 {
		return image.Config{}, err
	}

	// Expecting only one screenshot exist.
	imgFile := files[0]

	reader, err := os.Open(imgFile)
	if err != nil {
		return image.Config{}, errors.Wrap(err, "failed to open the screenshot")
	}
	defer reader.Close()

	imageConfig, _, err := image.DecodeConfig(reader)
	if err != nil {
		return image.Config{}, errors.Wrap(err, "failed to decode the screenshot")
	}

	return imageConfig, nil
}

func getCaptureSurfaceFullScreenBounds(ctx context.Context, tconn *chrome.TestConn) (coords.Size, error) {
	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to get the primary display info")
	}

	displayMode, err := displayInfo.GetSelectedMode()
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to obtain the display mode")
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		return coords.Size{}, errors.Wrap(err, "failed to obtain the display orientation")
	}

	expectedFullScreenshotSize := coords.NewSize(displayMode.WidthInNativePixels, displayMode.HeightInNativePixels)
	// The screen ui orientation might be different from the DUT's default orientation setting.
	// If the orientation angle is 90 or 270 degrees,
	// swap the width and height value to match the current screen ui orientation.
	if orientation.Angle == 90 || orientation.Angle == 270 {
		expectedFullScreenshotSize.Width, expectedFullScreenshotSize.Height = expectedFullScreenshotSize.Height, expectedFullScreenshotSize.Width
	}

	return expectedFullScreenshotSize, nil
}

// DeleteAllScreenshots deletes all screenshot files first to verify the screenshot later.
func DeleteAllScreenshots(downloadsPath string) error {
	const screenshotPattern = "Screenshot*.png"
	files, err := filepath.Glob(filepath.Join(downloadsPath, screenshotPattern))
	if err != nil {
		return errors.Wrapf(err, "the pattern %q is malformed", screenshotPattern)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return errors.Wrap(err, "failed to delete the screenshot")
		}
	}

	return nil
}
