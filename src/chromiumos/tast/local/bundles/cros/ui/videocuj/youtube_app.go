// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	youtubePkg                   = "com.google.android.youtube"
	playerViewID                 = youtubePkg + ":id/player_view"
	qualityListItemID            = youtubePkg + ":id/list_item_text"
	uiWaitTime                   = 15 * time.Second // this is for arc-obj, not for uiauto.Context
	waitTimeAfterClickPlayerView = 3 * time.Second
)

var appStartTime time.Duration

// YtApp defines the members related to youtube app.
type YtApp struct {
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	a     *arc.ARC
	d     *androidui.Device
	video videoSrc
	act   *arc.Activity
}

// NewYtApp creates an instance of YtApp.
func NewYtApp(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, a *arc.ARC, d *androidui.Device, video videoSrc) *YtApp {
	return &YtApp{
		tconn: tconn,
		kb:    kb,
		a:     a,
		d:     d,
		video: video,
	}
}

// OpenAndPlayVideo opens a video on youtube app.
func (y *YtApp) OpenAndPlayVideo(ctx context.Context) (err error) {
	testing.ContextLog(ctx, "Open Youtube app")

	const (
		youtubeApp              = "Youtube App"
		youtubeAct              = "com.google.android.apps.youtube.app.WatchWhileActivity"
		accountImageID          = "com.google.android.youtube:id/image"
		searchButtonID          = "com.google.android.youtube:id/menu_item_1"
		searchEditTextID        = "com.google.android.youtube:id/search_edit_text"
		moreOptions             = "com.google.android.youtube:id/player_overflow_button"
		noThanksID              = "com.google.android.youtube:id/dismiss"
		accountImageDescription = "Account"
	)

	if appStartTime, y.act, err = cuj.OpenAppAndGetStartTime(ctx, y.tconn, y.a, youtubePkg, youtubeApp, youtubeAct); err != nil {
		return errors.Wrap(err, "failed to get app start time")
	}

	accountImage := y.d.Object(androidui.ID(accountImageID), androidui.DescriptionContains(accountImageDescription))
	if err := accountImage.WaitForExists(ctx, uiWaitTime); err != nil {
		return errors.Wrap(err, "failed to check for Youtube app launched")
	}

	// Clear notification prompt if it exists.
	noThanksEle := y.d.Object(androidui.ID(noThanksID), androidui.Text("NO THANKS"))
	if err := cuj.ClickIfExist(noThanksEle, 5*time.Second)(ctx); err != nil {
		return errors.Wrap(err, `failed to click "NO THANKS" to clear notification prompt`)
	}

	playVideo := func() error {
		testing.ContextLog(ctx, "Search and play video")

		searchButton := y.d.Object(androidui.ID(searchButtonID))
		if err := searchButton.Click(ctx); err != nil {
			return err
		}

		searchEditText := y.d.Object(androidui.ID(searchEditTextID))
		if err := cuj.FindAndClick(searchEditText, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, `failed to find "searchTextfield"`)
		}

		if err := uiauto.Combine("type video url",
			y.kb.TypeAction(y.video.url),
			y.kb.AccelAction("enter"),
		)(ctx); err != nil {
			return err
		}

		firstVideo := y.d.Object(androidui.ClassName("android.view.ViewGroup"), androidui.Index(1))
		startTime := time.Now()
		if err := testing.Poll(ctx, func(ctx context.Context) error {

			if err := cuj.FindAndClick(firstVideo, uiWaitTime)(ctx); err != nil {
				if strings.Contains(err.Error(), "click") {
					return testing.PollBreak(err)
				}
				return errors.Wrap(err, `failed to find "First Video"`)
			}

			testing.ContextLogf(ctx, "Elapsed time when waiting the video list: %.3f s", time.Since(startTime).Seconds())
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 30 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to click first video")
		}
		return nil
	}

	// Switch quality is a continuous action.
	// Dut to the different response time of DUTs.
	// We need to combine these actions in Poll to make switch quality works smoothly.
	switchQuality := func(resolution string) error {
		testing.ContextLogf(ctx, "Switch Quality to %q", resolution)
		startTime := time.Now()
		if err := testing.Poll(ctx, func(context.Context) error {
			playerView := y.d.Object(androidui.ID(playerViewID))
			if err := cuj.FindAndClick(playerView, uiWaitTime)(ctx); err != nil {
				return errors.Wrap(err, "failed to find/click the player view on switch quality")
			}

			moreBtn := y.d.Object(androidui.ID(moreOptions))
			if err := cuj.FindAndClick(moreBtn, waitTimeAfterClickPlayerView)(ctx); err != nil {
				return errors.Wrap(err, `failed to find/click the "More options"`)
			}

			qualityBtn := y.d.Object(androidui.Text("Quality"))
			if err := cuj.FindAndClick(qualityBtn, uiWaitTime)(ctx); err != nil {
				return errors.Wrap(err, "failed to find/click the Quality")
			}

			advancedBtn := y.d.Object(androidui.Text("Advanced"))
			if err := cuj.FindAndClick(advancedBtn, uiWaitTime)(ctx); err != nil {
				return errors.Wrap(err, "failed to find/click the advanced option")
			}

			targetQuality := y.d.Object(androidui.Text(resolution), androidui.ID(qualityListItemID))
			if err := targetQuality.WaitForExists(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to find the target quality")
			}

			// Immediately clicking the target button sometimes doesn't work.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep and wait before click resolution")
			}
			if err := targetQuality.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click the target quality")
			}

			testing.ContextLogf(ctx, "Elapsed time when switching quality: %.3f s", time.Since(startTime).Seconds())
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			return err
		}
		return nil
	}

	if err := playVideo(); err != nil {
		return errors.Wrap(err, "failed to play video")
	}

	if err := switchQuality(y.video.quality); err != nil {
		return errors.Wrap(err, "failed to switch Quality")
	}

	return nil
}

func (y *YtApp) checkYoutubeAppPIP(ctx context.Context) error {
	testing.ContextLog(ctx, "Check window state should be PIP")
	startTime := time.Now()

	ws, err := ash.GetARCAppWindowState(ctx, y.tconn, youtubePkg)
	if err != nil {
		return errors.Wrap(err, "can not get ARC App Window State")
	}
	if ws == ash.WindowStatePIP {
		testing.ContextLogf(ctx, "Elapsed time when checking PIP mode: %.3f s", time.Since(startTime).Seconds())
		return nil
	}

	waitForPipMode := func(ctx context.Context) error {
		return ash.WaitForARCAppWindowState(ctx, y.tconn, youtubePkg, ash.WindowStatePIP)
	}

	// Checking PIP mode sometimes doesn't work (e.g. if chrome window is not in fullscreen),
	// retry a few times to enable PIP mode.
	return uiauto.Retry(3,
		uiauto.Combine("change to pip mode",
			y.kb.AccelAction("Alt+="),
			waitForPipMode,
		),
	)(ctx)
}

// EnterFullscreen switches youtube video to fullscreen.
func (y *YtApp) EnterFullscreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Make Youtube app fullscreen")

	const fullscreenBtnID = "com.google.android.youtube:id/fullscreen_button"

	playerView := y.d.Object(androidui.ID(playerViewID))

	startTime := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := cuj.FindAndClick(playerView, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the player view")
		}

		fsBtn := y.d.Object(androidui.ID(fullscreenBtnID))
		if err := cuj.FindAndClick(fsBtn, waitTimeAfterClickPlayerView)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the fullscreen button")
		}

		testing.ContextLogf(ctx, "Elapsed time when doing enter fullscreen %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return ash.WaitForARCAppWindowState(ctx, y.tconn, youtubePkg, ash.WindowStateFullscreen)
}

// PauseAndPlayVideo verifies video playback on youtube app.
func (y *YtApp) PauseAndPlayVideo(ctx context.Context) error {
	testing.ContextLog(ctx, "Pause and play video")

	const (
		playPauseBtnID = "com.google.android.youtube:id/player_control_play_pause_replay_button"
		playBtnDesc    = "Play video"
		pauseBtnDesc   = "Pause video"
		sleepTime      = 3 * time.Second
	)

	playerView := y.d.Object(androidui.ID(playerViewID))
	pauseBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(pauseBtnDesc))
	playBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(playBtnDesc))

	startTime := time.Now()
	return testing.Poll(ctx, func(ctx context.Context) error {

		if err := cuj.FindAndClick(playerView, uiWaitTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the player view")
		}

		if err := cuj.FindAndClick(pauseBtn, waitTimeAfterClickPlayerView)(ctx); err != nil {
			return errors.Wrap(err, "failed to find/click the pause button")
		}

		if err := playBtn.WaitForExists(ctx, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the play button")
		}

		// Immediately clicking the target button sometimes doesn't work.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep before clicking play button")
		}
		if err := playBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the play button")
		}
		if err := pauseBtn.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, "failed to find the pause button")
		}

		// Keep the video playing for a short time.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep while video is playing")
		}

		testing.ContextLogf(ctx, "Elapsed time when checking the playback status of youtube app: %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute})
}

// Close closes the resources related to video.
func (y *YtApp) Close(ctx context.Context) {
	if y.act != nil {
		y.act.Stop(ctx, y.tconn)
		y.act.Close()
		y.act = nil
	}
}
