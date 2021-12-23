// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
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

	y.ytConn, err = y.cr.NewConn(ctx, y.video.url, browser.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "failed to open youtube")
	}

	if err := webutil.WaitForYoutubeVideo(ctx, y.ytConn, 0); err != nil {
		return errors.Wrap(err, "failed to wait for video element")
	}

	// If prompted to open in YouTube app, instruct device to stay in Chrome.
	stayInChrome := nodewith.Name("Stay in Chrome").Role(role.Button)
	if err := y.ui.IfSuccessThen(
		y.ui.WithTimeout(5*time.Second).WaitUntilExists(stayInChrome),
		func(ctx context.Context) error {
			testing.ContextLog(ctx, "dialog popped up and asked whether to switch to YouTube app")
			rememberMyChoice := nodewith.Name("Remember my choice").Role(role.CheckBox)
			if err := y.uiHdl.Click(rememberMyChoice)(ctx); err != nil {
				return err
			}
			if err := y.uiHdl.Click(stayInChrome)(ctx); err != nil {
				return err
			}
			testing.ContextLog(ctx, "instructed device to stay on YouTube web")
			return nil
		},
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to instruct device to stay on YouTube web")
	}

	// Clear notification prompts if exists. If the notification alert popup isn't clear,
	// operations that require finding the current active window (i.e., SwitchWindowToDisplay)
	// will not succeed.
	prompts := []string{"Allow", "Never", "NO THANKS"}
	clearNotificationPrompts(ctx, y.ui, y.uiHdl, prompts...)

	// Default expected display is main display.
	if err := cuj.SwitchWindowToDisplay(ctx, y.tconn, y.kb, y.extendedDisplay)(ctx); err != nil {
		if y.extendedDisplay {
			return errors.Wrap(err, "failed to switch Youtube to the extended display")
		}
		return errors.Wrap(err, "failed to switch Youtube to the main display")
	}

	skipAdButton := nodewith.NameStartingWith("Skip Ad").Role(role.Button)
	if err := y.ui.IfSuccessThen(y.ui.WaitUntilExists(skipAdButton), y.uiHdl.Click(skipAdButton))(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'Skip Ad' button")
	}

	switchQuality := func(resolution string) error {
		videoPlayer := nodewith.Name("YouTube Video Player").Role(role.GenericContainer)
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
			if err := y.uiHdl.Click(videoPlayer)(ctx); err != nil {
				return errors.Wrap(err, "failed to click YouTube Video Player to bring up settings panel")
			}

			// If an ad is playing, skip it before proceeding.
			if err := y.ui.IfSuccessThen(y.ui.WaitUntilExists(skipAdButton), y.uiHdl.Click(skipAdButton))(ctx); err != nil {
				return errors.Wrap(err, "failed to click 'Skip Ad' button")
			}

			if err := y.uiHdl.ClickUntil(settings, y.ui.WithTimeout(10*time.Second).WaitUntilExists(quality))(ctx); err != nil {
				if y.extendedDisplay {
					return errors.Wrap(err, "failed to show the setting panel and click it on extended display")
				}
				return errors.Wrap(err, "failed to show the setting panel and click it on internal display")
			}

			testing.ContextLogf(ctx, "Elapsed time to click setting panel: %.3f s", time.Since(startTime).Seconds())
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 30 * time.Second}); err != nil {
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

	// The video should be playing at this point. However, we'll double check to make sure
	// as we have seen a few cases where the video became paused automatically.
	if err := y.ui.IfSuccessThen(
		y.ui.WithTimeout(waitTime).WaitUntilExists(playBtn),
		y.uiHdl.Click(playBtn),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to ensure video is playing before pausing")
	}

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
	startTime := time.Now()
	// Wait for element to appear.
	return testing.Poll(ctx, func(ctx context.Context) error {
		// Querying the main <video> node in youtube page.
		var state bool
		if err := conn.Call(ctx, &state, `() => {
			let video = document.querySelector("#movie_player > div.html5-video-container > video");
			return video.readyState === 4 && video.buffered.length > 0;
		}`); err != nil {
			return err
		}
		if !state {
			return errors.New("failed to wait for youtube on ready state")
		}
		testing.ContextLogf(ctx, "Elapsed time when waiting for youtube ready state: %.3f s", time.Since(startTime).Seconds())
		return nil
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
