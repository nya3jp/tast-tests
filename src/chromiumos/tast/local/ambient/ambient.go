// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ambient supports interaction with ChromeOS Ambient mode.
package ambient

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/local/session"
)

// const for ambient topic sources.
const (
	GooglePhotos = "Google Photos"
	ArtGallery   = "Art gallery"
)

// const for ambient themes.
const (
	SlideShow     = "Slide show"
	FeelTheBreeze = "Feel the breeze"
	FloatOnBy     = "Float on by"
)

// Default values for TestParams' fields.
const (
	// Slide show mode only requires downloading and decoding 2 photos before the
	// screen saver starts rendering. The animations, however, can request around
	// 16 photos before starting (with an internal timeout after which screen
	// saver starts anyways if it can't prepare all 16). Since this is
	// significantly more than slide show, the timeout should be larger.
	AmbientStartSlideShowDefaultTimeout = 15 * time.Second
	AmbientStartAnimationDefaultTimeout = 30 * time.Second
	// Not used. Animation playback speed does not apply to slide show yet.
	SlideShowDefaultPlaybackSpeed = 1
	// Typical animation cycle duration is currently 60 seconds. 60 seconds / 20
	// = 3 second cycle duration. This should give the test ample time to iterate
	// through 2 full animation cycles, giving it sufficient test coverage.
	AnimationDefaultPlaybackSpeed = 20
)

// TestParams for each test case.
type TestParams struct {
	TopicSource            string
	Theme                  string
	AnimationPlaybackSpeed float32
	AnimationStartTimeout  time.Duration
}

// DeviceSettings that must be set on the DUT before the test begins. The main
// overarching purpose of these is to speed up ambient mode so that the test
// doesn't have to take too long.
type DeviceSettings struct {
	LockScreenIdle         time.Duration
	BackgroundLockScreen   time.Duration
	PhotoRefreshInterval   time.Duration
	AnimationPlaybackSpeed float32
}

func toNearestSecond(d time.Duration) int {
	return int(d.Round(time.Second).Seconds())
}

// SetDeviceSettings changes settings for Ambient mode to speed up testing.
// Rounds values to the nearest second.
func SetDeviceSettings(
	ctx context.Context,
	tconn *chrome.TestConn,
	deviceSettings DeviceSettings,
) error {
	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.lock_screen_idle_timeout",
		toNearestSecond(deviceSettings.LockScreenIdle),
	); err != nil {
		return errors.Wrap(err, "failed to set lock screen idle timeout")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.lock_screen_background_timeout",
		toNearestSecond(deviceSettings.BackgroundLockScreen),
	); err != nil {
		return errors.Wrap(err, "failed to set lock screen background timeout")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.photo_refresh_interval",
		toNearestSecond(deviceSettings.PhotoRefreshInterval),
	); err != nil {
		return errors.Wrap(err, "failed to set photo refresh interval")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.animation_playback_speed",
		deviceSettings.AnimationPlaybackSpeed,
	); err != nil {
		return errors.Wrap(err, "failed to set playback speed")
	}

	return nil
}

// toggleAmbientMode returns an action to toggle ambient mode in ambient subpage.
func toggleAmbientMode(currentMode string, ui *uiauto.Context) uiauto.Action {
	toggleAmbientButton := nodewith.Role(role.ToggleButton).Name(currentMode)
	return uiauto.Combine(fmt.Sprintf("toggle ambient mode - %s", currentMode),
		ui.WaitUntilExists(toggleAmbientButton),
		ui.LeftClick(toggleAmbientButton))
}

// EnableAmbientMode returns an action to open ambient subpage from
// personalization hub then enable ambient mode.
func EnableAmbientMode(ui *uiauto.Context) uiauto.Action {
	return uiauto.Combine("Open screensaver subpage and enable ambient mode",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenScreensaverSubpage(ui),
		toggleAmbientMode("Off", ui))
}

// waitForPhotoTransitions blocks until the desired number of photo transitions
// has occurred.
func waitForPhotoTransitions(
	ctx context.Context,
	tconn *chrome.TestConn,
	numCompletions int,
	timeout time.Duration,
) error {
	return tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.autotestPrivate.waitForAmbientPhotoAnimation)`,
		numCompletions,
		toNearestSecond(timeout),
	)
}

// TestLockScreenIdle performs screensaver test including locking screen, waiting for ambient mode
// to start, then escaping from ambient mode and returning to lockscreen again.
func TestLockScreenIdle(
	ctx context.Context,
	cr *chrome.Chrome,
	tconn *chrome.TestConn,
	ui *uiauto.Context,
	ambientStartTimeout time.Duration,
) error {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get session manager")
	}
	return uiauto.Combine("start, hide, and restart ambient mode",
		sm.LockScreen,
		waitForAmbientStart(tconn, ui, ambientStartTimeout),
		hideAmbientMode(tconn, sm, ui),
		waitForAmbientStart(tconn, ui, ambientStartTimeout),
	)(ctx)
}

// waitForAmbientStart returns an action to wait for ambient mode to start and validate
// the number of photo transitions during ambient mode.
func waitForAmbientStart(tconn *chrome.TestConn, ui *uiauto.Context, timeout time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		if err := waitForPhotoTransitions(
			ctx,
			tconn,
			2,
			timeout,
		); err != nil {
			return errors.Wrap(err, "failed to wait for photo transitions")
		}

		return ui.Exists(nodewith.ClassName("LockScreenAmbientModeContainer").Role(role.Window))(ctx)
	}
}

// hideAmbientMode returns an action to move the mouse to escape from ambient mode
// and return to lockscreen.
func hideAmbientMode(
	tconn *chrome.TestConn,
	sm *session.SessionManager,
	ui *uiauto.Context,
) uiauto.Action {
	return func(ctx context.Context) error {
		container := nodewith.ClassName("LockScreenAmbientModeContainer").Role(role.Window)
		if err := ui.Exists(container)(ctx); err != nil {
			return errors.Wrap(err, "failed to find lock screen ambient mode container")
		}

		// Move the mouse a small amount. Ambient mode should turn off. Session
		// should still be locked.
		mouse, err := input.Mouse(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get mouse")
		}
		defer mouse.Close()

		if err := mouse.Move(10, 10); err != nil {
			return errors.Wrap(err, "failed to move mouse")
		}

		// Ambient mode container should not exist.
		if err := ui.WaitUntilGone(container)(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure ambient container dismissed")
		}

		if err := ensureScreenLocked(ctx, sm); err != nil {
			return err
		}

		return nil
	}
}

// UnlockScreen enters the password to unlock screen.
func UnlockScreen(ctx context.Context, password string) error {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get session manager")
	}

	if err := ensureScreenLocked(ctx, sm); err != nil {
		return errors.Wrap(err, "expected screen to be locked before unlocking it")
	}

	// Enter password to unlock screen.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer kb.Close()

	if err := kb.Type(ctx, password+"\n"); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	return nil
}

func ensureScreenLocked(ctx context.Context, sm *session.SessionManager) error {
	// Session should be locked.
	if isLocked, err := sm.IsScreenLocked(ctx); err != nil {
		return errors.Wrap(err, "failed to get screen lock state")
	} else if !isLocked {
		return errors.New("expected screen to be locked")
	}
	return nil
}
