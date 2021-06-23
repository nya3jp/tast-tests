// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ambient contains tests for ChromeOS Ambient mode.
package ambient

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/ambient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		VarDeps:      []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
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

	ui := uiauto.New(tconn).WithPollOpts(*defaultOSSettingsPollOptions)

	if err := ui.Retry(2, func(ctx context.Context) error {
		return prepareAmbientMode(ctx, tconn, ui)
	})(ctx); err != nil {
		s.Fatal("Failed to prepare ambient mode: ", err)
	}
	defer func() {
		if err := cleanupAmbientMode(ctx, tconn); err != nil {
			s.Fatal("Failed to clean up ambient mode: ", err)
		}
	}()

	if err := testLockScreenIdle(ctx, cr, tconn, ui); err != nil {
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
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
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

func prepareAmbientMode(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	if err := ambient.SetEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to set ambient mode pref to false")
	}

	settingsPage, err := ossettings.LaunchAtPage(
		ctx,
		tconn,
		nodewith.Name("Personalization").Role(role.Link),
	)
	if err != nil {
		return errors.Wrap(err, "opening settings page failed")
	}
	defer settingsPage.Close(ctx)

	subMenuLink := nodewith.Name("Screen saver Disabled").Role(role.Link)
	toggleSwitch := nodewith.Name("Off").Role(role.ToggleButton)
	albumsButton := nodewith.Name("Select Art gallery albums").Role(role.Button)
	artAlbumFinder := nodewith.NameStartingWith("Album").Role(role.CheckBox)

	if err := uiauto.Combine("turn on ambient mode and view art albums",
		ui.LeftClick(subMenuLink),
		ui.WithTimeout(5*time.Second).LeftClick(toggleSwitch),
		ui.LeftClick(albumsButton),
		ui.WaitUntilExists(artAlbumFinder.First()),
	)(ctx); err != nil {
		return err
	}

	artAlbums, err := ui.NodesInfo(ctx, artAlbumFinder)
	if err != nil {
		return errors.Wrap(err, "failed to find art gallery albums")
	}
	if len(artAlbums) < 2 {
		return errors.New("At least 2 art gallery albums expected")
	}

	// Sleep briefly because artAlbum buttons may not be clickable yet.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	// Turn off all but one art gallery album.
	for i, album := range artAlbums[1:] {
		if !strings.HasSuffix(album.Name, "selected") {
			return errors.Errorf("Art album %d should start selected", i)
		}
		if err := ui.MouseClickAtLocation(0, album.Location.CenterPoint())(ctx); err != nil {
			return errors.Wrapf(err, "failed to deselect art album %d", i)
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
		return errors.Wrap(err, "failed to configure ambient timeouts")
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
		return errors.Wrap(err, "failed to configure ambient timeouts")
	}

	if err := ambient.SetEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to set ambient mode pref off")
	}

	return nil
}

func testLockScreenIdle(
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

func waitForAmbientStart(tconn *chrome.TestConn, ui *uiauto.Context) uiauto.Action {
	return func(ctx context.Context) error {
		if err := ambient.WaitForPhotoTransitions(
			ctx,
			tconn,
			2,
			8*time.Second,
		); err != nil {
			return errors.Wrap(err, "failed to wait for photo transitions")
		}

		return ui.Exists(nodewith.ClassName("LockScreenAmbientModeContainer").Role(role.Window))(ctx)
	}
}

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
		if err := ui.EnsureGoneFor(container, time.Second)(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure ambient container dismissed")
		}

		// Session should be locked.
		if isLocked, err := sm.IsScreenLocked(ctx); err != nil {
			return errors.Wrap(err, "failed to get screen lock state")
		} else if !isLocked {
			return errors.New("expected screen to be locked")
		}

		return nil
	}
}
