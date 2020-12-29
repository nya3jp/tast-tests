// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"fmt"
	"regexp"
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

// YtWeb defines the members related to youtube web.
type YtWeb struct {
	cr      *chrome.Chrome
	tconn   *chrome.TestConn
	kb      *input.KeyboardEventWriter
	video   Video
	ui      *uiauto.Context
	ytConn  *chrome.Conn
	ytWinID int
	uiHdl   cuj.UIActionHandler

	extendedDisplay bool
}

// NewYtWeb creates the instance of YtWeb.
func NewYtWeb(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, video Video,
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

	const timeout = 15 * time.Second
	ui := y.ui.WithTimeout(timeout)

	y.ytConn, err = y.cr.NewConn(ctx, y.video.url, cdputil.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "failed to open youtube")
	}

	if err := webutil.WaitForYoutubeVideo(ctx, y.ytConn, 0); err != nil {
		return errors.Wrap(err, "failed to wait for video element")
	}

	adsButton := nodewith.Name("Skip Ads").Role(role.Button)
	if err := uiauto.Combine("click 'Skip Ads' button",
		ui.WithTimeout(10*time.Second).WaitForLocation(adsButton),
		// The script does click the Skip Ads button but no response when youtube is on extended display.
		// It seems the same issue as clicking settings button in the next step.
		y.uiHdl.Click(adsButton),
	)(ctx); err != nil {
		testing.ContextLog(ctx, "There's no Ads button")
	}

	switchQuality := func(resolution string) error {
		videoPlayer := nodewith.Name("YouTube Video Player")
		playButton := nodewith.Name("Play (k)").Role(role.Button)
		pauseBtn := nodewith.Name("Pause (k)").Role(role.Button)
		settings := nodewith.Name("Settings").Role(role.PopUpButton)
		quality := nodewith.NameRegex(regexp.MustCompile(`^Quality`)).Role(role.MenuItem)

		if err := ui.WaitUntilExists(videoPlayer)(ctx); err != nil {
			return errors.Wrap(err, `failed to find "YouTube Video Player"`)
		}

		coords, err := ui.Location(ctx, settings)
		if err != nil {
			return err
		}
		testing.ContextLogf(ctx, "center coords of settings: %d", coords.CenterPoint())

		startTime := time.Now()
		// The setting panel will automatically disappear if it isn't receive any event after few seconds.
		// Dut to the different response time of DUTs.
		// We need to combine these actions in Poll to make switch quality works smoothly.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := uiauto.Combine("show the setting panel and click it",
				y.uiHdl.Click(videoPlayer),
				ui.WithTimeout(time.Second).WaitUntilExists(settings),
				ui.WaitForLocation(settings),
				// The script does click the settings button but no response when youtube is on extended display.
				y.uiHdl.Click(settings),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to show the setting panel and click it")
			}
			testing.ContextLogf(ctx, "Elapsed time to click setting panel: %.3f s", time.Since(startTime).Seconds())
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 20 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to click setting panel")
		}

		if err := y.uiHdl.Click(quality)(ctx); err != nil {
			return errors.Wrap(err, `failed to click "Quality"`)
		}

		testing.ContextLogf(ctx, "Click %q", resolution)

		resolutionFinder := nodewith.NameRegex(regexp.MustCompile("^" + resolution)).Role(role.MenuItemRadio)
		if err := y.uiHdl.Click(resolutionFinder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to click %q", resolution)
		}

		testing.ContextLog(ctx, "Verify youtube is ready to play")
		if err := waitForYoutubeReadyState(ctx, y.ytConn); err != nil {
			return errors.Wrap(err, "failed to wait for Youtube ready state")
		}

		// We've clicked the center of video player to show setting panel,
		// that might pause the video (mouse-click will, but touch-tap won't),
		// here let the video keep playing anyway when switch the quality is finished.
		if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(pauseBtn)(ctx); err != nil {
			return y.uiHdl.Click(playButton)(ctx)
		}

		return nil
	}

	// Clear notification prompts if exists.
	prompts := []string{"Allow", "Never", "NO THANKS"}
	clearNotificationPrompts(ctx, ui, y.uiHdl, prompts...)

	testing.ContextLog(ctx, "Switch to the new quality")
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
		return errors.New("already in fullscreen")
	}

	ui := y.ui.WithTimeout(time.Second)

	// Notification prompts are sometimes shown in fullscreen.
	prompts := []string{"Allow", "Never", "NO THANKS"}
	clearNotificationPrompts(ctx, ui, y.uiHdl, prompts...)

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
	const (
		playButton  = "Play (k)"
		pauseButton = "Pause (k)"
		timeout     = 15 * time.Second
		waitTime    = 3 * time.Second
	)

	pauseBtn := nodewith.Name(pauseButton).Role(role.Button)
	playBtn := nodewith.Name(playButton).Role(role.Button)

	ui := uiauto.New(y.tconn).WithTimeout(timeout)
	if err := uiauto.Combine("checking the playing status of youtube video",
		ui.WaitUntilExists(pauseBtn),
		y.uiHdl.Click(pauseBtn),
		ui.WaitUntilExists(playBtn),
		// Wait time to see the video is paused.
		ui.Sleep(waitTime),
		y.uiHdl.Click(playBtn),
		// Wait time to see the video is playing.
		ui.Sleep(waitTime),
	)(ctx); err != nil {
		return err
	}

	return nil
}

// waitForYoutubeReadyState does wait youtube video ready state then return.
func waitForYoutubeReadyState(ctx context.Context, conn *chrome.Conn) error {
	// VideoPlayer represents the main <video> node in youtube page.
	const VideoPlayer = "#movie_player > div.html5-video-container > video"
	queryCode := fmt.Sprintf("new Promise((resolve, reject) => { let video = document.querySelector(%q); resolve(video.readyState === 4 && video.buffered.length > 0); });", VideoPlayer)

	startTime := time.Now()
	// Wait for element to appear.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var pageState bool
		if err := conn.EvalPromise(ctx, queryCode, &pageState); err != nil {
			return err
		}
		if pageState {
			testing.ContextLogf(ctx, "Elapsed time when waiting for youtube ready state: %.3f s", time.Since(startTime).Seconds())
			return nil
		}
		return errors.New("failed to wait for youtube on ready state")
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
		return err
	}
	return nil
}

// Close closes the resources related to video.
func (y *YtWeb) Close(ctx context.Context) {
	y.ytConn.CloseTarget(ctx)
	y.ytConn.Close()
}
