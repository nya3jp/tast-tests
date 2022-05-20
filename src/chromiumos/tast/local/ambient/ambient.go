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
	OnStatus     = "On"
	OffStatus    = "Off"
)

// Timeouts contains durations to configure Ambient mode timeouts.
type Timeouts struct {
	LockScreenIdle       time.Duration
	BackgroundLockScreen time.Duration
	PhotoRefreshInterval time.Duration
}

func toNearestSecond(d time.Duration) int {
	return int(d.Round(time.Second).Seconds())
}

// SetTimeouts changes timeouts for Ambient mode to speed up testing. Rounds
// values to the nearest second.
func SetTimeouts(
	ctx context.Context,
	tconn *chrome.TestConn,
	timeouts Timeouts,
) error {
	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.lock_screen_idle_timeout",
		toNearestSecond(timeouts.LockScreenIdle),
	); err != nil {
		return errors.Wrap(err, "failed to set lock screen idle timeout")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.lock_screen_background_timeout",
		toNearestSecond(timeouts.BackgroundLockScreen),
	); err != nil {
		return errors.Wrap(err, "failed to set lock screen background timeout")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.photo_refresh_interval",
		toNearestSecond(timeouts.PhotoRefreshInterval),
	); err != nil {
		return errors.Wrap(err, "failed to set photo refresh interval")
	}

	return nil
}

// OpenAmbientSubpage returns an action to open Ambient subpage from Personalization Hub.
func OpenAmbientSubpage(ctx context.Context, ui *uiauto.Context) error {
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get mouse")
	}
	defer mouse.Close()

	if err := uiauto.Combine("open Ambient Subpage",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenScreensaverSubpage(ui),
		ui.WaitUntilExists(nodewith.Role(role.Button).HasClass("breadcrumb").Name("Screensaver")))(ctx); err != nil {
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
) error {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get session manager")
	}
	return uiauto.Combine("start, hide, and restart ambient mode",
		sm.LockScreen,
		waitForAmbientStart(tconn, ui),
		hideAmbientMode(tconn, sm, ui),
		waitForAmbientStart(tconn, ui),
	)(ctx)
}

// waitForAmbientStart returns an action to wait for ambient mode to start and validate
// the number of photo transitions during ambient mode.
// Relax photo transitions timeout to 15 seconds to reserve enough time for the animation
// as lockscreen can happen for few minutes.
func waitForAmbientStart(tconn *chrome.TestConn, ui *uiauto.Context) uiauto.Action {
	return func(ctx context.Context) error {
		if err := waitForPhotoTransitions(
			ctx,
			tconn,
			2,
			15*time.Second,
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
