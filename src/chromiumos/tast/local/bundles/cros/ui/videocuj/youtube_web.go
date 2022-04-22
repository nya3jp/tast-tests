// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"regexp"
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

const (
	mouseMoveDuration = 500 * time.Millisecond
	shortUITimeout    = 5 * time.Second
)

var (
	videoPlayer = nodewith.NameStartingWith("YouTube Video Player").Role(role.GenericContainer)
	video       = nodewith.Role(role.Video).Ancestor(videoPlayer)
)

// YtWeb defines the struct related to youtube web.
type YtWeb struct {
	br      *browser.Browser
	tconn   *chrome.TestConn
	kb      *input.KeyboardEventWriter
	video   VideoSrc
	ui      *uiauto.Context
	ytConn  *chrome.Conn
	ytWinID int
	uiHdl   cuj.UIActionHandler

	extendedDisplay bool
}

// NewYtWeb creates an instance of YtWeb.
func NewYtWeb(br *browser.Browser, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, video VideoSrc,
	extendedDisplay bool, ui *uiauto.Context, uiHdl cuj.UIActionHandler) *YtWeb {
	return &YtWeb{
		br:    br,
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

	y.ytConn, err = y.uiHdl.NewChromeTab(ctx, y.br, y.video.URL, true)
	if err != nil {
		return errors.Wrap(err, "failed to open youtube tab")
	}

	if err := webutil.WaitForYoutubeVideo(ctx, y.ytConn, 0); err != nil {
		return errors.Wrap(err, "failed to wait for video element")
	}

	// If prompted to open in YouTube app, instruct device to stay in Chrome.
	stayInChrome := nodewith.Name("Stay in Chrome").Role(role.Button)
	if err := uiauto.IfSuccessThen(
		y.ui.WithTimeout(shortUITimeout).WaitUntilExists(stayInChrome),
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
	if err := clearNotificationPrompts(ctx, y.ui, y.uiHdl, prompts...); err != nil {
		return errors.Wrap(err, "failed to clear notification prompts")
	}

	// Default expected display is main display.
	if err := cuj.SwitchWindowToDisplay(ctx, y.tconn, y.kb, y.extendedDisplay)(ctx); err != nil {
		if y.extendedDisplay {
			return errors.Wrap(err, "failed to switch Youtube to the extended display")
		}
		return errors.Wrap(err, "failed to switch Youtube to the main display")
	}

	if err := y.SkipAd()(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'Skip Ad' button")
	}

	switchQuality := func(resolution string) error {
		testing.ContextLog(ctx, "Switch audio quality to ", resolution)
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
			if err := y.SkipAd()(ctx); err != nil {
				return errors.Wrap(err, "failed to click 'Skip Ad' button")
			}

			// Use DoDefault to avoid fauilure on lacros (see bug b/229003599).
			if err := y.ui.DoDefault(settings)(ctx); err != nil {
				return errors.Wrap(err, "failed to call DoDefault on settings button")
			}
			if err := y.ui.WithTimeout(10 * time.Second).WaitUntilExists(quality)(ctx); err != nil {
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

		// Use DoDefault to avoid fauilure on lacros (see bug b/229003599).
		if err := y.ui.DoDefault(quality)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Quality'")
		}

		resolutionFinder := nodewith.NameStartingWith(resolution).Role(role.MenuItemRadio).Ancestor(videoPlayer)
		if err := y.ui.DoDefault(resolutionFinder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to click %q", resolution)
		}

		if err := waitForYoutubeReadyState(ctx, y.ytConn); err != nil {
			return errors.Wrap(err, "failed to wait for Youtube ready state")
		}

		// We've clicked the center of video player to show setting panel,
		// that might pause the video (mouse-click will, but touch-tap won't),
		// here let the video keep playing anyway when switch the quality is finished.
		if err := uiauto.IfSuccessThen(
			y.ui.WithTimeout(3*time.Second).WaitUntilExists(video),
			y.uiHdl.Click(video),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure video is playing after show setting panel")
		}

		return nil
	}

	if err := switchQuality(y.video.Quality); err != nil {
		return errors.Wrapf(err, "failed to switch resolution to %s", y.video.Quality)
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
	if err := y.ui.DoDefault(fullscreenBtn)(ctx); err != nil {
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

// SkipAd skips the ad.
func (y *YtWeb) SkipAd() uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Checking for YouTube ads")

		adText := nodewith.NameContaining("Ad").Role(role.StaticText).Ancestor(videoPlayer).First()
		skipAdButton := nodewith.NameStartingWith("Skip Ad").Role(role.Button)
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := y.ui.WithTimeout(shortUITimeout).WaitUntilExists(adText)(ctx); err != nil {
				testing.ContextLog(ctx, "No ads found")
				return nil
			}
			if err := y.ui.Exists(skipAdButton)(ctx); err != nil {
				return errors.Wrap(err, "'Skip Ads' button not available yet")
			}
			if err := y.uiHdl.Click(skipAdButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to click 'Skip Ads'")
			}
			return errors.New("have not determined whether the ad has been skipped successfully")
		}, &testing.PollOptions{Timeout: time.Minute})
	}
}

// MaximizeWindow maximizes the youtube video.
func (y *YtWeb) MaximizeWindow(ctx context.Context) error {
	testing.ContextLog(ctx, "Maximize Youtube video window")

	if ytWin, err := ash.GetWindow(ctx, y.tconn, y.ytWinID); err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	} else if ytWin.State == ash.WindowStateMaximized {
		return nil
	}

	maximizeButton := nodewith.Name("Maximize").HasClass("FrameCaptionButton").Role(role.Button)
	if err := y.uiHdl.Click(maximizeButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to maximize the window")
	}
	if err := ash.WaitForCondition(ctx, y.tconn, func(w *ash.Window) bool {
		return w.ID == y.ytWinID && w.State == ash.WindowStateMaximized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for window to become maximized")
	}
	return nil
}

// MinimizeWindow minimizes the youtube video.
func (y *YtWeb) MinimizeWindow(ctx context.Context) error {
	testing.ContextLog(ctx, "Minimize Youtube video window")

	if ytWin, err := ash.GetWindow(ctx, y.tconn, y.ytWinID); err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	} else if ytWin.State == ash.WindowStateMinimized {
		return nil
	}

	minimizeButton := nodewith.Name("Minimize").HasClass("FrameCaptionButton").Role(role.Button)
	if err := y.uiHdl.Click(minimizeButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to minimize the window")
	}
	if err := ash.WaitForCondition(ctx, y.tconn, func(w *ash.Window) bool {
		return w.ID == y.ytWinID && w.State == ash.WindowStateMinimized && !w.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for window to become minimized")
	}
	return nil
}

// RestoreWindow restores the youtube video to normal state.
func (y *YtWeb) RestoreWindow(ctx context.Context) error {
	testing.ContextLog(ctx, "Restore Youtube video window")

	if _, err := ash.SetWindowState(ctx, y.tconn, y.ytWinID, ash.WMEventNormal, true /* waitForStateChange */); err != nil {
		return errors.Wrap(err, "failed to set the window state to normal")
	}
	if err := ash.WaitForCondition(ctx, y.tconn, func(w *ash.Window) bool {
		return w.ID == y.ytWinID && w.State == ash.WindowStateNormal && !w.IsAnimating
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for window to become normal")
	}
	return nil
}

// PauseAndPlayVideo verifies video playback on youtube web.
func (y *YtWeb) PauseAndPlayVideo(ctx context.Context) error {
	testing.ContextLog(ctx, "Pause and play video")

	// The video should be playing at this point. However, we'll double check to make sure
	// as we have seen a few cases where the video became paused automatically.
	if err := y.Play()(ctx); err != nil {
		return errors.Wrap(err, "failed to play the video")
	}

	return uiauto.Combine("check the playing status of youtube video",
		y.SkipAd(),
		y.Pause(),
		y.Play(),
	)(ctx)
}

// Play plays the video by clicking the video itself. If the video has already started playing, the function does nothing.
func (y *YtWeb) Play() uiauto.Action {
	return func(ctx context.Context) error {
		if err := y.IsPlaying()(ctx); err != nil {
			actionName := "play video"
			return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
				y.ui.MouseMoveTo(video, mouseMoveDuration),
				y.ui.LeftClick(video),
			))(ctx)
		}
		return nil
	}
}

// Pause pauses the video by clicking the video itself.
func (y *YtWeb) Pause() uiauto.Action {
	return func(ctx context.Context) error {
		if err := y.IsPaused()(ctx); err != nil {
			actionName := "pause video"
			return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
				y.ui.MouseMoveTo(video, mouseMoveDuration),
				y.ui.LeftClick(video),
			))(ctx)
		}
		return nil
	}
}

// IsPlaying checks if the video is playing now.
func (y *YtWeb) IsPlaying() uiauto.Action {
	return func(ctx context.Context) error {
		previousTime, err := y.getCurrentTime(ctx)
		if err != nil {
			return err
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			currentTime, err := y.getCurrentTime(ctx)
			if err != nil {
				return err
			}
			if currentTime > previousTime {
				return nil
			}
			return errors.New("youtube is not playing")
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}
}

// IsPaused checks if the video is paused now.
func (y *YtWeb) IsPaused() uiauto.Action {
	return func(ctx context.Context) error {
		previousTime, err := y.getCurrentTime(ctx)
		if err != nil {
			return err
		}
		// Considering that the paused action might not react immediately to the low-end DUTs.
		// If the "paused" reaches the threshold, it means the video is actually paused.
		threshold := 5
		paused := 0
		return testing.Poll(ctx, func(ctx context.Context) error {
			currentTime, err := y.getCurrentTime(ctx)
			if err != nil {
				return err
			}
			if currentTime == previousTime {
				paused++
			}
			if paused >= threshold {
				return nil
			}
			return errors.New("youtube is not paused")
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}
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

// getCurrentTime gets the current video time in seconds.
func (y *YtWeb) getCurrentTime(ctx context.Context) (int, error) {
	settings := nodewith.Name("Settings").Role(role.PopUpButton).Ancestor(videoPlayer)
	timeNode := nodewith.NameRegex(regexp.MustCompile("((\\d+):)?(\\d+):(\\d+)$")).Role(role.InlineTextBox).First()
	if err := uiauto.Combine("make youtube video play time pop up",
		y.ui.MouseMoveTo(settings, mouseMoveDuration),
		y.ui.MouseMoveTo(videoPlayer, mouseMoveDuration),
		y.ui.WaitUntilExists(timeNode),
	)(ctx); err != nil {
		return 0, err
	}

	node, err := y.ui.Info(ctx, timeNode)
	if err != nil {
		return 0, err
	}

	timeFormat := "4:05"
	if len(node.Name) > 5 { // If the playback time exceeds an hour.
		timeFormat = "15:04:05"
	}
	videoTime, err := time.Parse(timeFormat, node.Name)
	if err != nil {
		return 0, err
	}
	return videoTime.Hour()*60*60 + videoTime.Minute()*60 + videoTime.Second(), nil
}

// clearNotificationPrompts finds and clears some youtube web prompts.
func clearNotificationPrompts(ctx context.Context, ui *uiauto.Context, uiHdl cuj.UIActionHandler, prompts ...string) error {
	for _, name := range prompts {
		tartgetPrompt := nodewith.Name(name).Role(role.Button)
		if err := uiauto.IfSuccessThen(
			ui.WaitUntilExists(tartgetPrompt),
			uiHdl.ClickUntil(tartgetPrompt, ui.WithTimeout(shortUITimeout).WaitUntilGone(tartgetPrompt)),
		)(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to clear prompt %q", name)
			return err
		}
	}
	return nil
}

// PerformFrameDropsTest checks for dropped frames percent and checks if it is below the threshold.
func (y *YtWeb) PerformFrameDropsTest(ctx context.Context) error {
	// If we see more than 10% video frame drops it will be visible to the user and will impact the viewing experience.
	const frameDropThreshold float64 = 10.0
	var decodedFrameCount, droppedFrameCount int
	videoElement := "document.querySelector('#movie_player video')"

	if err := y.ytConn.Eval(ctx, videoElement+".getVideoPlaybackQuality().totalVideoFrames", &decodedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get decoded framecount")
	}
	if err := y.ytConn.Eval(ctx, videoElement+".getVideoPlaybackQuality().droppedVideoFrames", &droppedFrameCount); err != nil {
		return errors.Wrap(err, "failed to get dropped framecount")
	}
	droppedFramePercent := 100.0
	if decodedFrameCount != 0 {
		droppedFramePercent = 100.0 * float64(droppedFrameCount/decodedFrameCount)

	}
	if droppedFramePercent > frameDropThreshold {
		return errors.Errorf("frame drops rate %.2f (dropped %d, decoded %d) higher than allowed threshold %.2f", droppedFramePercent, droppedFrameCount, decodedFrameCount, frameDropThreshold)
	}
	return nil
}

// YtWebConn returns connection of youtube web.
func (y *YtWeb) YtWebConn() *chrome.Conn {
	return y.ytConn
}
