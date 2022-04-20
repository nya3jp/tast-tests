// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/ambient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
	})
}

func SetGooglePhotosScreensaver(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(chrome.Creds{
			User: s.RequiredVar("ambient.username"),
			Pass: s.RequiredVar("ambient.password"),
		}),
		chrome.EnableFeatures("PersonalizationHub"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Force Chrome to be in clamshell mode to make sure wallpaper preview is not
	// enabled.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := uiauto.Combine("Open screensaver subpage and enable ambient mode",
		personalization.OpenPersonalizationHub(ui),
		personalization.OpenScreensaverSubpage(ui),
		personalization.EnableAmbientMode(ui))(ctx); err != nil {
		s.Fatal("Failed to enable ambient mode: ", err)
	}

	if err := prepareAmbientMode(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to prepare ambient mode")
	}

	if err := ambient.TestLockScreenIdle(ctx, cr, tconn, ui); err != nil {
		s.Fatal("Failed to start ambient mode: ", err)
	}

	if err := ambient.UnlockScreen(ctx, s.RequiredVar("ambient.password")); err != nil {
		s.Fatal("Failed to unlock screen")
	}
}

func prepareAmbientMode(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	const containerName = "Select Google Photos Choose your favorite"
	googlePhotosContainer := nodewith.Role(role.GenericContainer).NameContaining(containerName)
	googleAlbumsFinder := nodewith.Role(role.GenericContainer).HasClass("album")

	if err := uiauto.Combine("Choose Google Photos topic source",
		ui.LeftClick(googlePhotosContainer),
		ui.WaitUntilExists(googleAlbumsFinder.First()))(ctx); err != nil {
		errors.Wrap(err, "failed to select Google Photos")
	}

	googlePhotosAlbums, err := ui.NodesInfo(ctx, googleAlbumsFinder)
	if err != nil {
		errors.Wrap(err, "failed to find Google Photos albums")
	}
	if len(googlePhotosAlbums) < 2 {
		errors.Wrap(err, "at least 2 Google Photos albums expected")
	}

	// Sleep briefly because googlePhotosAlbums buttons may not be clickable yet.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		errors.Wrap(err, "failed to sleep")
	}

	// Select all Google Photos albums.
	for i, album := range googlePhotosAlbums {
		if err := ui.MouseClickAtLocation(0, album.Location.CenterPoint())(ctx); err != nil {
			errors.Wrapf(err, "failed to select google photo album %d", i)
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
