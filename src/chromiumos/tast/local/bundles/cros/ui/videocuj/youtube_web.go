// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// YtWeb defines the struct related to youtube web.
type YtWeb struct {
	cr      *chrome.Chrome
	tconn   *chrome.TestConn
	kb      *input.KeyboardEventWriter
	video   videoSrc
	ui      *uiauto.Context
	ytConn  *chrome.Conn
	ytWinID int
	uiHdl   cuj.UIActionHandler

	extendedDisplay bool
}

// NewYtWeb creates an instance of YtWeb.
func NewYtWeb(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, video videoSrc,
	extendedDisplay bool, ui *uiauto.Context, uiHdl cuj.UIActionHandler) *YtWeb {
	return &YtWeb{
		cr:    cr,
		tconn: tconn,
		kb:    kb,
		video: video,
		ui:    ui,
		uiHdl: uiHdl,

		extendedDisplay: extendedDisplay,
	}
}

// OpenAndPlayVideo opens a youtube video on chrome.
func (y *YtWeb) OpenAndPlayVideo(ctx context.Context) (err error) {
	testing.ContextLog(ctx, "Open Youtube web")

	y.ytConn, err = y.cr.NewConn(ctx, y.video.url, cdputil.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "failed to open youtube")
	}

	if err := webutil.WaitForYoutubeVideo(ctx, y.ytConn, 0); err != nil {
		return errors.Wrap(err, "failed to wait for video element")
	}

	// Root window on built-in display.
	targetWin := nodewith.ClassName("RootWindow-0").Role(role.Window)
	if y.extendedDisplay {
		// Root window on extended display.
		targetWin = nodewith.ClassName("RootWindow-1").Role(role.Window)
		ytPlayerView := nodewith.Name("YouTube Video Player").Ancestor(targetWin)
		if err := y.ui.Exists(ytPlayerView)(ctx); err != nil {
			// Chrome is not on extended display.
			testing.ContextLog(ctx, "Switch Youtube to extended display")
			if err := y.kb.Accel(ctx, "Search+Alt+M"); err != nil {
				return errors.Wrap(err, "failed to switch Youtube to the extended display: ")
			}
		}
	}

	adsButton := nodewith.Name("Skip Ads").Role(role.Button)
	if err := y.ui.IfSuccessThen(y.ui.WaitUntilExists(adsButton), y.uiHdl.Click(adsButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to find ads and skip ads")
	}

	switchQuality := func(resolution string) error {
		videoPlayer := nodewith.Name("YouTube Video Player").Ancestor(targetWin)
		playButton := nodewith.Name("Play (k)").Role(role.Button).Ancestor(videoPlayer)
		settings := nodewith.Name("Settings").Role(role.PopUpButton).Ancestor(videoPlayer)
		quality := nodewith.NameStartingWith("Quality").Role(role.MenuItem).Ancestor(videoPlayer)

		if err := y.ui.WaitUntilExists(videoPlayer)(ctx); err != nil {
			return errors.Wrap(err, "failed to find 'YouTube Video Player'")
		}

		startTime := time.Now()
		// The setting panel will automatically disappear if it does not receive any event after a few seconds.
		// Dut to the different response time of different DUTs, we need to combine these actions in Poll() to
		// make quality switch works reliably.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := uiauto.Combine("show the setting panel and click it",
				y.uiHdl.Click(videoPlayer),
				y.uiHdl.ClickUntil(settings, y.ui.WithTimeout(10*time.Second).WaitUntilExists(quality)),
			)(ctx); err != nil {
				if y.extendedDisplay {
					return errors.Wrap(err, "failed to show the setting panel and click it on extended display")
				}
				return errors.Wrap(err, "failed to show the setting panel and click it on internal display")
			}
			testing.ContextLogf(ctx, "Elapsed time to click setting panel: %.3f s", time.Since(startTime).Seconds())
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 20 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to click setting panel")
		}

		if err := y.uiHdl.Click(quality)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Quality'")
		}

		resolutionFinder := nodewith.NameStartingWith(resolution).Role(role.MenuItemRadio).Ancestor(videoPlayer)
		if err := y.uiHdl.Click(resolutionFinder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to click %q", resolution)
		}

		if err := waitForYoutubeReadyState(ctx, y.ytConn); err != nil {
			return errors.Wrap(err, "failed to wait for Youtube ready state")
		}

		// We've clicked the center of video player to show setting panel,
		// that might pause the video (mouse-click will, but touch-tap won't),
		// here let the video keep playing anyway when switch the quality is finished.
		if err := y.ui.IfSuccessThen(
			y.ui.WithTimeout(3*time.Second).WaitUntilExists(playButton),
			y.uiHdl.Click(playButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure video is playing after show setting panel")
		}

		return nil
	}

	// Clear notification prompts if exists.
	prompts := []string{"Allow", "Never", "NO THANKS"}
	clearNotificationPrompts(ctx, y.ui, y.uiHdl, prompts...)

	if err := switchQuality(y.video.quality); err != nil {
		return errors.Wrapf(err, "failed to switch resolution to %s", y.video.quality)
	}

	y.ytWinID, err = getWindowID(ctx, y.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get window ID")
	}

	return nil
}

// EnterFullscreen switches youtube video to fullscreen.
func (y *YtWeb) EnterFullscreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Make Youtube video fullscreen")

	if ytWin, err := ash.GetWindow(ctx, y.tconn, y.ytWinID); err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	} else if ytWin.State == ash.WindowStateFullscreen {
		return nil
	}

	// Notification prompts are sometimes shown in fullscreen.
	prompts := []string{"Allow", "Never", "NO THANKS"}
	clearNotificationPrompts(ctx, y.ui, y.uiHdl, prompts...)

	fullscreenBtn := nodewith.Name("Full screen (f)").Role(role.Button)
	if err := y.uiHdl.Click(fullscreenBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click fullscreen button")
	}

	if err := waitWindowStateFullscreen(ctx, y.tconn, y.ytWinID); err != nil {
		return errors.Wrap(err, "failed to tap fullscreen button")
	}

	if err := waitForYoutubeReadyState(ctx, y.ytConn); err != nil {
		return errors.Wrap(err, "failed to wait for Youtube ready state")
	}
	return nil
}

// PauseAndPlayVideo verifies video playback on youtube web.
func (y *YtWeb) PauseAndPlayVideo(ctx context.Context) error {
	testing.ContextLog(ctx, "Pause and play video")

	const (
		playButton  = "Play (k)"
		pauseButton = "Pause (k)"
		timeout     = 15 * time.Second
		waitTime    = 3 * time.Second
	)

	pauseBtn := nodewith.Name(pauseButton).Role(role.Button)
	playBtn := nodewith.Name(playButton).Role(role.Button)

	ui := uiauto.New(y.tconn).WithTimeout(timeout)
	return uiauto.Combine("check the playing status of youtube video",
		ui.WaitUntilExists(pauseBtn),
		y.uiHdl.Click(pauseBtn),
		ui.WaitUntilExists(playBtn),
		// Keep in paused state for a while.
		ui.Sleep(waitTime),
		y.uiHdl.Click(playBtn),
		// Keep playing for a while.
		ui.Sleep(waitTime),
	)(ctx)
}

// waitForYoutubeReadyState does wait youtube video ready state then return.
func waitForYoutubeReadyState(ctx context.Context, conn *chrome.Conn) error {
	// VideoPlayer represents the main <video> node in youtube page.
	const VideoPlayer = "#movie_player > div.html5-video-container > video"
	queryCode := fmt.Sprintf("new Promise((resolve, reject) => { let video = document.querySelector(%q); resolve(video.readyState === 4 && video.buffered.length > 0); });", VideoPlayer)

	startTime := time.Now()
	// Wait for element to appear.
	return testing.Poll(ctx, func(ctx context.Context) error {
		var pageState bool
		if err := conn.EvalPromiseDeprecated(ctx, queryCode, &pageState); err != nil {
			return err
		}
		if pageState {
			testing.ContextLogf(ctx, "Elapsed time when waiting for youtube ready state: %.3f s", time.Since(startTime).Seconds())
			return nil
		}
		return errors.New("failed to wait for youtube on ready state")
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute})
}

// Close closes the resources related to video.
func (y *YtWeb) Close(ctx context.Context) {
	if y.ytConn != nil {
		y.ytConn.CloseTarget(ctx)
		y.ytConn.Close()
		y.ytConn = nil
	}
}

// clearNotificationPrompts finds and clears some youtube web prompts.
// No error is returned because failing to clear the notification doesn't impact the test.
func clearNotificationPrompts(ctx context.Context, ui *uiauto.Context, uiHdl cuj.UIActionHandler, prompts ...string) {
	for _, name := range prompts {
		tartgetPrompt := nodewith.Name(name).Role(role.Button)
		if err := ui.WithTimeout(15*time.Second).IfSuccessThen(
			ui.WithTimeout(3*time.Second).WaitUntilExists(tartgetPrompt),
			uiHdl.ClickUntil(tartgetPrompt, ui.WithTimeout(2*time.Second).WaitUntilGone(tartgetPrompt)),
		)(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to clear prompt %q", name)
		}
	}
}
