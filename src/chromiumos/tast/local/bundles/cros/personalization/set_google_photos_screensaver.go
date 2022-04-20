// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/ambient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetGooglePhotosScreensaver,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting Google Photos screensaver in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
		Fixture:      "personalizationWithGaiaLogin",
	})
}

func SetGooglePhotosScreensaver(ctx context.Context, s *testing.State) {
	fixtData := s.FixtValue().(personalization.FixtData)
	cr := fixtData.Chrome
	tconn := fixtData.TestAPIConn

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := ambient.EnableAmbientMode(ui)(ctx); err != nil {
		s.Fatal("Failed to enable ambient mode: ", err)
	}

	if err := prepareGooglePhotosScreensaver(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to prepare Google Photos screensaver: ", err)
	}

	if err := ambient.TestLockScreenIdle(ctx, cr, tconn, ui); err != nil {
		s.Fatal("Failed to start ambient mode: ", err)
	}

	if err := ambient.UnlockScreen(ctx, s.RequiredVar("ambient.password")); err != nil {
		s.Fatal("Failed to unlock screen")
	}
}

func prepareGooglePhotosScreensaver(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	googlePhotosContainer := nodewith.Role(role.GenericContainer).NameContaining(ambient.GooglePhotos)
	googleAlbumsFinder := nodewith.Role(role.GenericContainer).HasClass("album")

	if err := uiauto.Combine("Choose Google Photos topic source",
		ui.LeftClick(googlePhotosContainer),
		ui.WaitUntilExists(googleAlbumsFinder.First()))(ctx); err != nil {
		return errors.Wrap(err, "failed to select Google Photos")
	}

	googlePhotosAlbums, err := ui.NodesInfo(ctx, googleAlbumsFinder)
	if err != nil {
		return errors.Wrap(err, "failed to find Google Photos albums")
	}
	if len(googlePhotosAlbums) < 2 {
		return errors.New("At least 2 Google Photos albums expected")
	}

	// Select all Google Photos albums.
	for i, album := range googlePhotosAlbums {
		if strings.Contains(album.ClassName, "album-selected") {
			return errors.Errorf("Google Photos album %d should be unselected", i)
		}
		selectedAlbumNode := nodewith.HasClass("album-selected").Name(album.Name)
		if err := ui.RetryUntil(uiauto.Combine("select Google Photo album",
			ui.Gone(selectedAlbumNode),
			ui.MouseClickAtLocation(0, album.Location.CenterPoint())),
			ui.WaitUntilExists(selectedAlbumNode))(ctx); err != nil {
			return errors.Wrapf(err, "failed to select Google Photos album %d", i)
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
