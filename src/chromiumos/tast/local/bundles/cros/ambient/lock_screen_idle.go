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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LockScreenIdle,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Locks the screen and starts Ambient mode",
		Contacts:     []string{"cowmoo@chromium.org", "wutao@chromium.org", "assistive-eng@google.com", "chromeos-sw-engprod@google.com"},
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

	if err := ambient.TestLockScreenIdle(ctx, cr, tconn, ui); err != nil {
		s.Fatal("Failed to start ambient mode: ", err)
	}

	if err := ambient.UnlockScreen(ctx, s.RequiredVar("ambient.password")); err != nil {
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
