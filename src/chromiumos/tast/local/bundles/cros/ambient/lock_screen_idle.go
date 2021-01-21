// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ambient contains tests for ChromeOS Ambient mode.
package ambient

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/ambient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockScreenIdle,
		Desc:         "Locks the screen and starts Ambient mode",
		Contacts:     []string{"cowmoo@chromium.org", "wutao@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

var defaultOSSettingsPollOptions = &testing.PollOptions{
	Timeout:  10 * time.Second,
	Interval: 1 * time.Second,
}

// LockScreenIdle prepares Ambient mode settings, locks the screen, and tests
// that Ambient mode downloads and displays photos.
func LockScreenIdle(ctx context.Context, s *testing.State) {
	cr, tconn, err := setup(
		ctx,
		s.RequiredVar("ambient.username"),
		s.RequiredVar("ambient.password"),
	)
	if err != nil {
		s.Fatal("Failed to complete setup: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := prepareAmbientMode(ctx, tconn); err != nil {
		s.Fatal("Failed to prepare ambient mode: ", err)
	}
	defer func() {
		if err := cleanupAmbientMode(ctx, tconn); err != nil {
			s.Fatal("Failed to clean up ambient mode: ", err)
		}
	}()

	if err := testLockScreenIdle(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to start ambient mode: ", err)
	}

	if err := unlockScreen(ctx, s.RequiredVar("ambient.password")); err != nil {
		s.Fatal("Failed to unlock screen")
	}
}

func setup(
	ctx context.Context,
	username,
	password string,
) (*chrome.Chrome, *chrome.TestConn, error) {
	cr, err := chrome.New(
		ctx,
		chrome.Auth(username, password, ""),
		chrome.GAIALogin(),
		chrome.EnableFeatures("ChromeOSAmbientMode:FineArtAlbumEnabled/true/CulturalInstitutePhotosEnabled/true/FeaturedPhotoAlbumEnabled/true/FeaturedPhotosEnabled/true"),
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start Chrome")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating test API connection failed")
	}

	return cr, tconn, nil
}

func prepareAmbientMode(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ambient.SetEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to set ambient mode pref to false")
	}

	if err := ossettings.LaunchAtPage(
		ctx,
		tconn,
		ui.FindParams{Name: "Personalization", Role: ui.RoleTypeLink},
	); err != nil {
		return errors.Wrap(err, "opening settings page failed")
	}

	if err := ui.StableFindAndClick(
		ctx,
		tconn,
		ui.FindParams{Name: "Screen saver Disabled", Role: ui.RoleTypeLink},
		defaultOSSettingsPollOptions,
	); err != nil {
		return errors.Wrap(err, "opening screen saver menu failed")
	}

	ambientToggle, err := ui.FindWithTimeout(
		ctx,
		tconn,
		ui.FindParams{Role: ui.RoleTypeToggleButton, Name: "Off"},
		5*time.Second,
	)
	if err != nil {
		return errors.Wrap(err, "finding ambient mode toggle failed")
	}
	defer ambientToggle.Release(ctx)

	if err := ambientToggle.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "toggling ambient mode failed")
	}

	if err := ui.StableFindAndClick(
		ctx,
		tconn,
		ui.FindParams{
			Role: ui.RoleTypeButton,
			Name: "Select Art gallery albums",
		},
		defaultOSSettingsPollOptions,
	); err != nil {
		return errors.Wrap(err, "clicking on art gallery radio button failed")
	}

	artAlbumParams := ui.FindParams{
		Role:       ui.RoleTypeCheckBox,
		Attributes: map[string]interface{}{"name": regexp.MustCompile("^Album.+")},
	}

	if err := ui.WaitUntilExists(
		ctx,
		tconn,
		artAlbumParams,
		defaultOSSettingsPollOptions.Timeout,
	); err != nil {
		return errors.Wrap(err, "failed waiting for art gallery albums")
	}

	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	artAlbums, err := ui.FindAll(ctx, tconn, artAlbumParams)
	if err != nil {
		return errors.Wrap(err, "finding art gallery albums failed")
	}
	defer artAlbums.Release(ctx)

	// Turn off all but one art gallery albums.
	for _, artAlbum := range artAlbums[1:] {
		if err := artAlbum.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "deselecting art album failed")
		}
	}

	if err := ambient.SetTimeouts(
		ctx,
		tconn,
		ambient.Timeouts{
			LockScreenIdle:       1 * time.Second,
			BackgroundLockScreen: 2 * time.Second,
			PhotoRefreshInterval: 1 * time.Second,
		},
	); err != nil {
		return errors.Wrap(err, "unable to configure ambient timeouts")
	}

	return nil
}

func unlockScreen(ctx context.Context, password string) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open keyboard device")
	}
	defer ew.Close()

	if err := ew.Type(ctx, password+"\n"); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	return nil
}

func cleanupAmbientMode(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ambient.SetTimeouts(
		ctx,
		tconn,
		ambient.Timeouts{
			LockScreenIdle:       7 * time.Second,
			BackgroundLockScreen: 5 * time.Second,
			PhotoRefreshInterval: 60 * time.Second,
		},
	); err != nil {
		return errors.Wrap(err, "unable to configure ambient timeouts")
	}

	if err := ambient.SetEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "unable to set ambient mode pref off")
	}

	return nil
}

func testLockScreenIdle(
	ctx context.Context,
	cr *chrome.Chrome,
	tconn *chrome.TestConn,
) error {
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get session manager")
	}

	if err := sm.LockScreen(ctx); err != nil {
		return errors.Wrap(err, "failed to lock screen")
	}

	if err := waitForAmbientStart(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start ambient mode")
	}

	if err := hideAmbientMode(ctx, tconn, sm); err != nil {
		return errors.Wrap(err, "failed to hide ambient mode")
	}

	// Wait for ambient mode to start again.
	if err := waitForAmbientStart(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start ambient mode after mouse move")
	}

	return nil
}

func waitForAmbientStart(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ambient.WaitForPhotoTransitions(
		ctx,
		tconn,
		2,
		8*time.Second,
	); err != nil {
		return errors.Wrap(err, "failed to wait for photo transitions")
	}

	if exists, err := ui.Exists(
		ctx,
		tconn,
		ui.FindParams{
			ClassName: "LockScreenAmbientModeContainer",
			Role:      ui.RoleTypeWindow,
		},
	); err != nil {
		return errors.Wrap(err, "failed to check if ambient container exists")
	} else if !exists {
		return errors.New("expected ambient mode to be on")
	}

	return nil
}

func hideAmbientMode(
	ctx context.Context,
	tconn *chrome.TestConn,
	sm *session.SessionManager,
) error {
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

	// Session should be locked.
	if isLocked, err := sm.IsScreenLocked(ctx); err != nil {
		return errors.Wrap(err, "failed to get screen lock state")
	} else if !isLocked {
		return errors.New("expected screen to be locked")
	}

	// Ambient mode container should not exist.
	if exists, err := ui.Exists(
		ctx,
		tconn,
		ui.FindParams{
			ClassName: "LockScreenAmbientModeContainer",
			Role:      ui.RoleTypeWindow,
		},
	); err != nil {
		return errors.Wrap(err, "failed to check if ambient container exists")
	} else if exists {
		return errors.New("expected ambient mode to be off")
	}

	return nil
}
