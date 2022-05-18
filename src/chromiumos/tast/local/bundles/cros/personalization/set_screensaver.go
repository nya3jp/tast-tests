// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package personalization

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/ambient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/personalization"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetScreensaver,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test setting screensaver in the personalization hub app",
		Contacts: []string{
			"thuongphan@google.com",
			"chromeos-sw-engprod@google.com",
			"assistive-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ambient.username", "ambient.password"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "personalizationWithGaiaLogin",
		Params: []testing.Param{
			{
				Name: "google_photos",
				Val:  ambient.GooglePhotos,
			},
			{
				Name: "art_gallery",
				Val:  ambient.ArtGallery,
			},
		},
	})
}

func SetScreensaver(ctx context.Context, s *testing.State) {
	topicSource := s.Param().(string)
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// The test has a dependency of network speed, so we give uiauto.Context ample
	// time to wait for nodes to load.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	if err := ambient.OpenAmbientSubpage(ctx, ui); err != nil {
		s.Fatal("Failed to open Ambient Subpage: ", err)
	}

	if err := ambient.EnableAmbientMode(ctx, ui); err != nil {
		s.Fatal("Failed to enable ambient mode: ", err)
	}

	if err := prepareScreensaver(ctx, tconn, ui, topicSource); err != nil {
		s.Fatalf("Failed to prepare %v screensaver: %v", topicSource, err)
	}

	if err := ambient.TestLockScreenIdle(ctx, cr, tconn, ui); err != nil {
		s.Fatal("Failed to start ambient mode: ", err)
	}

	if err := ambient.UnlockScreen(ctx, tconn, s.RequiredVar("ambient.username"), s.RequiredVar("ambient.password")); err != nil {
		s.Fatal("Failed to unlock screen: ", err)
	}
}

func prepareScreensaver(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, topicSource string) error {
	topicSourceContainer := nodewith.Role(role.GenericContainer).NameContaining(topicSource)
	albumsFinder := nodewith.Role(role.GenericContainer).HasClass("album")

	if err := uiauto.Combine("Choose topic source",
		ui.FocusAndWait(topicSourceContainer),
		ui.LeftClick(topicSourceContainer),
		ui.WaitUntilExists(albumsFinder.First()))(ctx); err != nil {
		return errors.Wrapf(err, "failed to select %v", topicSource)
	}

	albums, err := ui.NodesInfo(ctx, albumsFinder)
	if err != nil {
		return errors.Wrapf(err, "failed to find %v albums", topicSource)
	}
	if len(albums) < 2 {
		return errors.Errorf("at least 2 %v albums expected", topicSource)
	}

	if topicSource == ambient.GooglePhotos {
		// Select all Google Photos albums.
		for i, album := range albums {
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
	} else if topicSource == ambient.ArtGallery {
		// Turn off all but one art gallery album.
		for i, album := range albums[1:] {
			if !strings.Contains(album.ClassName, "album-selected") {
				return errors.Errorf("Art album %d should be selected", i)
			}
			selectedAlbumNode := nodewith.HasClass("album-selected").Name(album.Name)
			if err := ui.RetryUntil(uiauto.Combine("deselect Art Gallery album",
				ui.Exists(selectedAlbumNode),
				ui.MouseClickAtLocation(0, album.Location.CenterPoint())),
				ui.WaitUntilGone(selectedAlbumNode))(ctx); err != nil {
				return errors.Wrapf(err, "failed to deselect Art Gallery album %d", i)
			}
		}
	} else {
		return errors.Errorf("topicSource - %v is invalid", topicSource)
	}

	// Close Personalization Hub after ambient mode setup is finished.
	if err := personalization.ClosePersonalizationHub(ui)(ctx); err != nil {
		return errors.Wrap(err, "failed to close Personalization Hub")
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
