// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/youtube"
	"chromiumos/tast/local/personalization"
)

// const for ambient topic sources.
const (
	GooglePhotos = "Google Photos"
	ArtGallery   = "Art gallery"
	OnStatus     = "On"
	OffStatus    = "Off"
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

// TestVideoSrc contains an example Youtube video to play for testing
// "media string" in ambient mode. Taken from the BasicYoutubeCUJ test.
var TestVideoSrc = youtube.VideoSrc{
	URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
	Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
	Quality: "1080p60",
}

// TestParams for each test case.
type TestParams struct {
	TopicSource            string
	Theme                  string
	AnimationPlaybackSpeed float32
	AnimationStartTimeout  time.Duration
}

// DeviceSettings that must be set on the DUT before the test begins. These
// settings are not user-visible and can only be changed by test code. The main
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

// OpenAmbientSubpage returns an action to open Ambient subpage from Personalization Hub.
func OpenAmbientSubpage(ctx context.Context, ui *uiauto.Context) error {
	if err := uiauto.Combine("open Ambient Subpage",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenScreensaverSubpage(ui),
		ui.WaitUntilExists(personalization.BreadcrumbNodeFinder(personalization.ScreensaverSubpageName)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open Ambient subpage")
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

// ambientModeEnabled checks whether ambient mode is on.
func ambientModeEnabled(ctx context.Context, ui *uiauto.Context) (bool, error) {
	return ui.IsNodeFound(ctx, nodewith.Role(role.ToggleButton).Name(OnStatus))
}

// EnableAmbientMode enables ambient mode in Personalization Hub from Ambient Subpage.
// If ambient mode is already enabled, it does nothing.
func EnableAmbientMode(ctx context.Context, ui *uiauto.Context) error {
	ambientMode, err := ambientModeEnabled(ctx, ui)
	if err != nil {
		return errors.Wrap(err, "failed to check ambient mode status")
	}
	if !ambientMode {
		if err := toggleAmbientMode(OffStatus, ui)(ctx); err != nil {
			return errors.Wrap(err, "failed to enable ambient mode")
		}
	}
	return nil
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
	return uiauto.Combine("start, hide, and restart ambient mode",
		lockScreen(ctx, tconn),
		waitForAmbientStart(tconn, ui, ambientStartTimeout),
		hideAmbientMode(tconn, ui),
		waitForAmbientStart(tconn, ui, ambientStartTimeout),
	)(ctx)
}

// lockScreen returns an action to lock screen.
func lockScreen(ctx context.Context, tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		if err := lockscreen.Lock(ctx, tconn); err != nil {
			errors.Wrap(err, "failed to lock the screen")
		}
		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
			errors.Errorf("failed to wait for screen to be locked: %v (last status %+v)", err, st)
		}
		return nil
	}
}

// UnlockScreen enters the password to unlock screen.
func UnlockScreen(ctx context.Context, tconn *chrome.TestConn, username, password string) error {
	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer kb.Close()

	if err := lockscreen.EnterPassword(ctx, tconn, username, password, kb); err != nil {
		return errors.Wrap(err, "failed to unlock the screen")
	}
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		return errors.Errorf("failed to wait for screen to be unlocked: %v (last status %+v)", err, st)
	}
	return nil
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
		// For the media string condition:
		// .First() is needed because there are actually 2 media string "nodes"
		// present in the UI tree during slideshow mode. The second node is
		// invisible in the background and is a product of how slideshow mode is
		// implemented. Without ".First()"" though, this condition fails saying
		// that here are multiple matching nodes.
		if err := uiauto.Combine("validate ambient screen and media string",
			ui.WaitUntilExists(nodewith.ClassName("LockScreenAmbientModeContainer").Role(role.Window)),
			ui.WaitUntilExists(nodewith.NameContaining(TestVideoSrc.Title).First()),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to validate ambient screen and media string exist")
		}
		return nil
	}
}

// hideAmbientMode returns an action to move the mouse to escape from ambient mode
// and return to lockscreen.
func hideAmbientMode(
	tconn *chrome.TestConn,
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

		if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
			errors.Errorf("failed to wait for screen to be locked: %v (last status %+v)", err, st)
		}

		return nil
	}
}
