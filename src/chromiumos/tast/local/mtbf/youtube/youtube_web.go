// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package youtube

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	mouseMoveDuration = 500 * time.Millisecond
	longUITimeout     = time.Minute
	shortUITimeout    = 5 * time.Second
)

var (
	videoPlayer = nodewith.NameStartingWith("YouTube Video Player").Role(role.GenericContainer)
	video       = nodewith.Role(role.Video).Ancestor(videoPlayer)
	videoButton = nodewith.Role(role.Button).Ancestor(videoPlayer).NameRegex(regexp.MustCompile("^(Pause|Play).*"))
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
	// Lacros will focus on the search bar after navigating, press Enter to make sure the focus is on the webarea.
	searchBar := nodewith.Role(role.TextField).Name("Address and search bar").Focused()
	if err := uiauto.IfSuccessThen(y.ui.Exists(searchBar), y.kb.AccelAction("Enter"))(ctx); err != nil {
		return err
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
			rememberReg := regexp.MustCompile("Remember (my|this) choice")
			rememberChoice := nodewith.NameRegex(rememberReg).Role(role.CheckBox)
			if err := y.uiHdl.Click(rememberChoice)(ctx); err != nil {
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

	if err := clearNotificationPrompts(ctx, y.ui); err != nil {
		return errors.Wrap(err, "failed to clear notification prompts")
	}

	// Use keyboard to play/pause video and ensure PageLoad.PaintTiming.NavigationToLargestContentfulPaint2
	// can be generated correctly. See b/240998447.
	if err := uiauto.Combine("pause and play with keyboard",
		y.Pause(),
		y.Play(),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to pause and play before switching quality")
	}

	// Sometimes prompts to grant permission appears after opening a video for a while.
	if err := clearNotificationPrompts(ctx, y.ui); err != nil {
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

	if err := y.SwitchQuality(y.video.Quality)(ctx); err != nil {
		return errors.Wrapf(err, "failed to switch resolution to %s", y.video.Quality)
	}

	y.ytWinID, err = getFirstWindowID(ctx, y.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get window ID")
	}

	// Ensure the video is playing.
	return uiauto.IfFailThen(y.IsPlaying(), y.Play())(ctx)
}

// SwitchQuality switches youtube quality.
func (y *YtWeb) SwitchQuality(resolution string) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switch video quality to ", resolution)
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

		// Keep the video playing anyway when switch the quality is finished.
		return uiauto.IfFailThen(y.IsPlaying(), y.Play())(ctx)
	}
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
	if err := clearNotificationPrompts(ctx, y.ui); err != nil {
		return errors.Wrap(err, "failed to clear notification prompts")
	}

	fullscreenBtn := nodewith.Name("Full screen (f)").Role(role.Button)
	if err := y.ui.DoDefault(fullscreenBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click fullscreen button")
	}

	if err := waitWindowStateFullscreen(ctx, y.tconn, YoutubeWindowTitle); err != nil {
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
		}, &testing.PollOptions{Timeout: longUITimeout})
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
	return uiauto.NamedCombine("pause and play video",
		y.SkipAd(),
		// The video should be playing at this point. However, we'll double check to make sure
		// as we have seen a few cases where the video became paused automatically.
		uiauto.IfFailThen(y.IsPlaying(), y.Play()),
		y.Pause(),
		y.Play(),
	)(ctx)
}

// Play returns a function to play the video.
func (y *YtWeb) Play() uiauto.Action {
	return uiauto.IfSuccessThen(y.IsPaused(), uiauto.NamedCombine("play video",
		y.ui.WithTimeout(longUITimeout).RetryUntil(y.kb.TypeAction("k"), y.IsPlaying())))
}

// Pause returns a function to pause the video.
func (y *YtWeb) Pause() uiauto.Action {
	return uiauto.IfSuccessThen(y.IsPlaying(), uiauto.NamedCombine("pause video",
		y.ui.WithTimeout(longUITimeout).RetryUntil(y.kb.TypeAction("k"), y.IsPaused())))
}

// StartCast casts YouTube video to a specified screen connected to ADT-3.
func (y *YtWeb) StartCast(accessCode string) uiauto.Action {
	accessCodeTextField := nodewith.Name("Type the access code to start casting").Role(role.TextField).Editable()
	incorrectPasswordText := nodewith.Name("You've entered an incorrect access code. Try again.").Role(role.StaticText)

	enterCastCode := uiauto.NamedCombine("enter the access code",
		y.kb.TypeAction(accessCode),
		y.kb.AccelAction("Enter"),
	)

	return uiauto.NamedCombine("start casting the video",
		quicksettings.StartCast(y.tconn),
		y.ui.WaitUntilExists(accessCodeTextField),
		enterCastCode,
		uiauto.IfSuccessThen(y.ui.WithTimeout(shortUITimeout).WaitUntilExists(incorrectPasswordText),
			uiauto.Combine("input access code again",
				y.kb.AccelAction("Ctrl+A"),
				y.kb.AccelAction("Backspace"),
				enterCastCode,
			),
		),
		y.Pause(),
		y.Play(),
	)
}

// StopCast stops casting YouTube video.
func (y *YtWeb) StopCast() uiauto.Action {
	return uiauto.NamedAction("stop casting the video", quicksettings.StopCast(y.tconn))
}

// ResetCastStatus resets the cast settings if the YouTube video is already casting.
func (y *YtWeb) ResetCastStatus() uiauto.Action {
	youtubeWindow := nodewith.NameContaining("YouTube").Role(role.Window).HasClass("Widget")
	customizeChromeButton := nodewith.Name("Chrome").Role(role.PopUpButton).Ancestor(youtubeWindow)
	castDialog := nodewith.NameStartingWith("Cast").Role(role.AlertDialog).Ancestor(youtubeWindow)
	availableButton := nodewith.NameContaining("Available").Role(role.Button).Ancestor(castDialog).First()
	return uiauto.NamedCombine("reset to available",
		y.uiHdl.Click(customizeChromeButton),
		uiauto.IfSuccessThen(y.ui.WithTimeout(shortUITimeout).WaitUntilExists(availableButton), y.uiHdl.Click(availableButton)),
	)
}

const (
	playButton  = "Play (k)"
	pauseButton = "Pause (k)"
)

func (y *YtWeb) getPlayButtonTitle(ctx context.Context) (result string, err error) {
	script := `document.querySelector(".ytp-play-button").title`
	if err := y.ytConn.Eval(ctx, script, &result); err != nil {
		return result, errors.Wrap(err, "failed to get result")
	}
	return result, nil
}

// IsPlaying checks if the video is playing now.
func (y *YtWeb) IsPlaying() uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			title, err := y.getPlayButtonTitle(ctx)
			if err != nil {
				return err
			}
			if title == pauseButton {
				testing.ContextLog(ctx, "Youtube is playing")
				return nil
			}
			return errors.Errorf("youtube is not playing; got (%s)", title)
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}
}

// IsPaused checks if the video is paused now.
func (y *YtWeb) IsPaused() uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			title, err := y.getPlayButtonTitle(ctx)
			if err != nil {
				return err
			}
			if title == playButton {
				testing.ContextLog(ctx, "Youtube is paused")
				return nil
			}
			return errors.Errorf("youtube is not paused; got (%s)", title)
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
	}, &testing.PollOptions{Interval: time.Second, Timeout: longUITimeout})
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
func clearNotificationPrompts(ctx context.Context, ui *uiauto.Context) error {
	tartgetPrompts := nodewith.NameRegex(regexp.MustCompile("(Allow|Never|NO THANKS)")).Role(role.Button)
	nodes, err := ui.NodesInfo(ctx, tartgetPrompts)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	testing.ContextLog(ctx, "Start to clear notification prompts")
	prompts := []string{"Allow", "Never", "NO THANKS"}
	for _, name := range prompts {
		tartgetPrompt := nodewith.Name(name).Role(role.Button)
		if err := uiauto.IfSuccessThen(
			ui.WithTimeout(shortUITimeout).WaitUntilExists(tartgetPrompt),
			ui.DoDefaultUntil(tartgetPrompt, ui.WithTimeout(shortUITimeout).WaitUntilGone(tartgetPrompt)),
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
