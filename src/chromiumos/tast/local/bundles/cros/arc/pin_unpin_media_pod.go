// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/youtube"
	"chromiumos/tast/local/arc/apputil/youtubemusic"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PinUnpinMediaPod,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check the pin/unpin/re-pin for media control pod",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"ting.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc"},
		// There are two apps to be installed in this case.
		Timeout: 2*time.Minute + 2*apputil.InstallationTimeout,
		Fixture: "arcBootedWithPlayStore",
	})
}

const (
	ytAppLink       = "https://www.youtube.com/watch?v=JE3-LkMqBfM"
	ytAppVideo      = "Whale Songs and AI, for everyone to explore"
	ytMusicVideo    = "Beat It"
	ytMusicSubtitle = "Michael Jackson • 4:19"
)

// PinUnpinMediaPod checks the pin/unpin/re-pin for media control pod.
func PinUnpinMediaPod(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var currentMediaName string
	for appName, media := range map[string]*apputil.Media{
		youtubemusic.AppName: apputil.NewMedia(ytMusicVideo, ytMusicSubtitle),
		youtube.AppName:      apputil.NewMedia(ytAppLink, ytAppVideo),
	} {
		var err error
		var app apputil.ARCMediaPlayer
		switch appName {
		case youtube.AppName:
			currentMediaName = media.Subtitle
			app, err = youtube.NewApp(ctx, kb, tconn, a)
		case youtubemusic.AppName:
			currentMediaName = media.Query
			app, err = youtubemusic.New(ctx, kb, tconn, a)
		default:
			s.Fatal("Failed to create media app instance: unexpected media source: ", appName)
		}
		if err != nil {
			s.Fatal("Failed to create media app instance: ", err)
		}

		if err := app.Install(ctx); err != nil {
			s.Fatal("Failed to install: ", err)
		}

		if _, err := app.Launch(ctx); err != nil {
			s.Fatal("Failed to launch: ", err)
		}
		defer app.Close(cleanupCtx, cr, s.HasError, filepath.Join(s.OutDir(), appName))

		if err := app.Play(ctx, media); err != nil {
			s.Fatal("Failed to play media: ", err)
		}
	}

	// Media controls pod is pinned by default, unpin it for the test.
	if err := quicksettings.UnpinMediaControlsPod(tconn)(ctx); err != nil {
		s.Fatal("Failed to ensure media controls pod is unpinned: ", err)
	}

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings: ", err)
	}
	defer quicksettings.Hide(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_quicksettings")

	ui := uiauto.New(tconn)

	// currentMediaName is used to enter the media control pod detail view,
	// where the pin button and all media control panels are located.
	if err := pinAndVerify(ui, tconn, currentMediaName)(ctx); err != nil {
		s.Fatal("Failed to pin and verify: ", err)
	}

	if err := unpinAndVerify(ui, tconn)(ctx); err != nil {
		s.Fatal("Failed to unpin and verify: ", err)
	}

	if err := pinAndVerify(ui, tconn, currentMediaName)(ctx); err != nil {
		s.Fatal("Failed to pin again and verify: ", err)
	}
}

// unpinAndVerify unpins media pod and verify it is appeared in quick settings.
func unpinAndVerify(ui *uiauto.Context, tconn *chrome.TestConn) uiauto.Action {
	dialogView := nodewith.Ancestor(quicksettings.MediaControlsDialog)

	return uiauto.Combine("unpin and find media pod in quick settings",
		ui.LeftClick(quicksettings.PinnedMediaControls),
		ui.WaitUntilExists(dialogView.Role(role.ListItem).NameStartingWith(ytMusicVideo)),
		ui.WaitUntilExists(dialogView.Role(role.ListItem).NameStartingWith(ytAppVideo)),
		quicksettings.UnpinMediaControlsPod(tconn),
		reopenQuickSettings(tconn),
		ui.WaitUntilExists(quicksettings.MediaControlsPod),
	)
}

// pinAndVerify verifies both media control panels are inside detail view,
// then pins media pod and verifies it is disappeared in quick settings.
func pinAndVerify(ui *uiauto.Context, tconn *chrome.TestConn, title string) uiauto.Action {
	detailView := nodewith.Ancestor(quicksettings.MediaControlsDetailView)

	return uiauto.Combine("pin media pod and verify it is disappeared in quick settings",
		quicksettings.NavigateToMediaControlsSubpage(tconn, title),
		ui.WaitUntilExists(detailView.Role(role.ListItem).NameStartingWith(ytMusicVideo)),
		ui.WaitUntilExists(detailView.Role(role.ListItem).NameStartingWith(ytAppVideo)),
		quicksettings.PinMediaControlsPod(tconn),
		reopenQuickSettings(tconn),
		ui.WaitUntilGone(quicksettings.MediaControlsPod),
	)
}

// reopenQuickSettings reopens quick settings.
func reopenQuickSettings(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		if err := quicksettings.Hide(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to hide quicksettings")
		}
		return quicksettings.Show(ctx, tconn)
	}
}
