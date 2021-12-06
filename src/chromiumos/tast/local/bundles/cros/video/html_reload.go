// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/video"
	"chromiumos/tast/testing"
)

const (
	mp4URL  = "shaka_dash_480.mp4"
	webmURL = "shaka_dash_480_vp8.webm"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HTMLReload,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks video playback functionalities after reloading webpage",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{mp4URL, webmURL},
		Fixture:      "chromeLoggedIn",
	})
}

// HTMLReload verifies video playback functionalities after reloading webpage.
func HTMLReload(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for video, url := range map[string]string{
		mp4URL:  s.DataPath(mp4URL),
		webmURL: s.DataPath(webmURL),
	} {
		dataPath, err := setupImportedTargets(video, url)
		if err != nil {
			s.Fatal("Failed to setup imported targets: ", err)
		}
		defer os.Remove(dataPath)
		s.Log("Data path: ", dataPath)

		videoPlayer := newVideoPlayer(tconn, fmt.Sprintf("file:///%s", dataPath))
		testDesc := fmt.Sprintf("verify video playback functionalities of %q", video)

		f := func(ctx context.Context, s *testing.State) {
			cleanupSubtestCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			s.Logf("Opening video: %s", video)
			if err := videoPlayer.Open(ctx, cr); err != nil {
				s.Fatal("Failed to open video: ", err)
			}
			defer func(ctx context.Context) {
				faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("ui_dump_%s", video))
				videoPlayer.Close(ctx)
			}(cleanupSubtestCtx)

			if err := videoTest(ctx, videoPlayer); err != nil {
				s.Fatal("Failed to test video functionalities: ", err)
			}
		}

		if !s.Run(ctx, testDesc, f) {
			s.Error("Failed to complete test of ", testDesc)
		}
	}
}

func videoTest(ctx context.Context, player *videoPlayer) error {
	if isPlaying, err := player.IsPlaying(ctx); err != nil {
		return errors.Wrap(err, "failed to verify video is playing")
	} else if !isPlaying {
		return errors.New("video is not playing")
	}

	const playDuration = 10 * time.Second

	testing.ContextLogf(ctx, "Let the video play for at least %s", playDuration)
	if err := testing.Sleep(ctx, playDuration); err != nil {
		return errors.Wrapf(err, "failed to let the video play for at least %s", playDuration)
	}

	timeBeforeReload, err := player.CurrentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get currentTime before page reloaded")
	}

	testing.ContextLog(ctx, "Reloading the page")
	if err := player.reloadPage(ctx); err != nil {
		return errors.Wrap(err, "failed reload the page")
	}

	if err := player.WaitUntilVideoReady(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until video is ready")
	}

	if isPlaying, err := player.IsPlaying(ctx); err != nil {
		return errors.Wrap(err, "failed to verify video is playing")
	} else if !isPlaying {
		return errors.New("video is not playing")
	}

	timeAfterReload, err := player.CurrentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get currentTime after page reloaded")
	}

	if timeAfterReload >= timeBeforeReload {
		return errors.Errorf("video did not replay from the beginning after reloading the webpage: video time {Before: %f, After: %f}", timeBeforeReload, timeAfterReload)
	}

	testing.ContextLog(ctx, "Verifying basic functionalities of the video")
	return uiauto.Combine("verify html video basic functionalities",
		uiauto.NamedAction("pause video", player.Pause),
		uiauto.NamedAction("play video", player.Play),
		uiauto.NamedAction("fast-forward video", player.Forward),
		uiauto.NamedAction("fast-rewind video", player.Rewind),
		uiauto.NamedAction("turn on full screen", player.EnterFullScreen),
		uiauto.NamedAction("turn off full screen", player.ExitFullScreen),
	)(ctx)
}

type videoPlayer struct {
	browserRoot  *nodewith.Finder
	playerFinder *nodewith.Finder

	*video.Video
	ui *uiauto.Context
}

func newVideoPlayer(tconn *chrome.TestConn, url string) *videoPlayer {
	var (
		browserRoot    = nodewith.NameStartingWith("Chrome").HasClass("BrowserFrame").Role(role.Window)
		playerFinder   = nodewith.HasClass("phase-ready").Role(role.GenericContainer).FinalAncestor(browserRoot)
		playerSelector = "document.querySelector('video')"
	)

	return &videoPlayer{
		browserRoot:  browserRoot,
		playerFinder: playerFinder,
		Video:        video.New(tconn, url, playerSelector, playerFinder),
		ui:           uiauto.New(tconn),
	}
}

func (v *videoPlayer) reloadPage(ctx context.Context) error {
	reloadBtn := nodewith.Name("Reload").HasClass("ReloadButton").Role(role.Button).FinalAncestor(v.browserRoot)
	return uiauto.Combine("reload webpage",
		v.ui.LeftClick(reloadBtn),
		v.ui.WaitUntilExists(v.playerFinder),
	)(ctx)
}

// setupImportedTargets imported given files to `Downloads` folder.
func setupImportedTargets(name, source string) (string, error) {
	pathInDownloads := filepath.Join(filesapp.DownloadPath, name)
	if err := fsutil.CopyFile(source, pathInDownloads); err != nil {
		return "", errors.Wrapf(err, "failed to copy file to folder %s", pathInDownloads)
	}
	return pathInDownloads, nil
}
