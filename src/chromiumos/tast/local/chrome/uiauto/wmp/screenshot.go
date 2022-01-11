// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmp contains utility functions for window management and performance.
package wmp

import (
	"context"
	"regexp"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type screenshotType int

const (
	// FullScreen is the type to take a full screenshot.
	FullScreen screenshotType = iota
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
		case FullScreen:
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
