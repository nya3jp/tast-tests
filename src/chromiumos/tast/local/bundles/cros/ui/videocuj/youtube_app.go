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
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	youtubePkg                   = "com.google.android.youtube"
	playerViewID                 = youtubePkg + ":id/player_view"
	qualityListItemID            = youtubePkg + ":id/list_item_text"
	uiWaitTime                   = 15 * time.Second
	waitTimeAfterClickPlayerView = 3 * time.Second
)

var appStartTime time.Duration

// YtApp defines the members related to youtube app.
type YtApp struct {
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	a     *arc.ARC
	d     *androidui.Device
	video Video
	act   *arc.Activity
}

// NewYtApp creates an instance of YtApp.
func NewYtApp(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, a *arc.ARC, d *androidui.Device, video Video) *YtApp {
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

	testing.ContextLog(ctx, "Open Youtube app")

	if appStartTime, y.act, err = cuj.OpenAppAndGetStartTime(ctx, y.tconn, y.a, youtubePkg, youtubeApp, youtubeAct); err != nil {
		return errors.Wrap(err, "failed to get app start time")
	}

	testing.ContextLog(ctx, "Wait for YouTube app to be launched")
	accountImage := y.d.Object(androidui.ID(accountImageID), androidui.DescriptionContains(accountImageDescription))
	if err := accountImage.WaitForExists(ctx, uiWaitTime); err != nil {
		return errors.Wrap(err, "failed to check for Youtube app launched")
	}

	// Skip "NO THANKS" if it shows up.
	noThanksEle := y.d.Object(androidui.ID(noThanksID), androidui.Text("NO THANKS"))
	if err := cuj.ClickIfExist(ctx, noThanksEle, 5*time.Second); err != nil {
		testing.ContextLog(ctx, "Click 'NO THANKS' failed")
	}

	playVideo := func() error {
		searchButton := y.d.Object(androidui.ID(searchButtonID))
		if err := searchButton.Click(ctx); err != nil {
			return err
		}

		testing.ContextLog(ctx, `Find 'searchTextField'`)
		searchEditText := y.d.Object(androidui.ID(searchEditTextID))
		if err := cuj.FindAndClick(ctx, searchEditText, uiWaitTime); err != nil {
			return errors.Wrap(err, `failed to find 'searchTextfield': `)
		}

		testing.ContextLog(ctx, "Typing video url")
		if err := y.kb.Type(ctx, y.video.url); err != nil {
			return err
		}
		if err := y.kb.Accel(ctx, "enter"); err != nil {
			return errors.Wrap(err, "failed to type the enter key")
		}

		firstVideo := y.d.Object(androidui.ClassName("android.view.ViewGroup"), androidui.Index(1))
		startTime := time.Now()
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			testing.ContextLog(ctx, "Wait the video list")

			if err := cuj.FindAndClick(ctx, firstVideo, uiWaitTime); err != nil {
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
		testing.ContextLog(ctx, "Switch Quality")
		startTime := time.Now()
		if err := testing.Poll(ctx, func(context.Context) error {
			testing.ContextLog(ctx, "Find player view and click it")
			playerView := y.d.Object(androidui.ID(playerViewID))
			if err := cuj.FindAndClick(ctx, playerView, uiWaitTime); err != nil {
				return errors.Wrap(err, "failed to find/click the player view on switch quality")
			}

			testing.ContextLog(ctx, "Find more button and click it")
			moreBtn := y.d.Object(androidui.ID(moreOptions))
			if err := cuj.FindAndClick(ctx, moreBtn, waitTimeAfterClickPlayerView); err != nil {
				return errors.Wrap(err, `failed to find/click the "More options"`)
			}

			testing.ContextLog(ctx, "Find the Quality button")
			qualityBtn := y.d.Object(androidui.Text("Quality"))
			if err := cuj.FindAndClick(ctx, qualityBtn, uiWaitTime); err != nil {
				return errors.Wrap(err, "failed to find/click the Quality")
			}

			testing.ContextLog(ctx, "Find the advanced option")
			advancedBtn := y.d.Object(androidui.Text("Advanced"))
			if err := cuj.FindAndClick(ctx, advancedBtn, uiWaitTime); err != nil {
				return errors.Wrap(err, "failed to find/click the advanced option")
			}

			testing.ContextLogf(ctx, "Find the target quality [%s]", resolution)
			targetQuality := y.d.Object(androidui.Text(resolution), androidui.ID(qualityListItemID))
			if err := targetQuality.WaitForExists(ctx, 2*time.Second); err != nil {
				return errors.Wrap(err, "failed to find the target quality")
			}
			// Immediately clicking the target button sometimes doesn't work.
			testing.Sleep(ctx, time.Second)
			if err := targetQuality.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click the target quality")
			}

			testing.ContextLogf(ctx, "Clicked target quality: %s", resolution)

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

func checkYoutubeAppPIP(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Check window state should be PIP")
	startTime := time.Now()
	// Checking PIP mode sometimes doesn't work if chrome window is not in fullscreen.
	// Giving the Polling time to re-check window state.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if ws, err := ash.GetARCAppWindowState(ctx, tconn, youtubePkg); err != nil {
			return errors.Wrap(err, "can not get ARC App Window State")
		} else if ws != ash.WindowStatePIP {
			testing.ContextLog(ctx, "Press alt + '='")
			if err := kb.AccelPress(ctx, "Alt"); err != nil {
				return errors.Wrap(err, "failed to press alt")
			}
			defer kb.AccelRelease(ctx, "Alt")
			// Hold time when pressing 'Alt'
			if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			if err := kb.Accel(ctx, "="); err != nil {
				return errors.Wrap(err, `failed to type "="`)
			}
			// Wait time for window to fullscreen.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			return errors.Wrap(err, "window state should be PIP")
		}
		testing.ContextLogf(ctx, "Elapsed time when checking PIP mode: %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

// EnterFullscreen switches youtube video to fullscreen.
func (y *YtApp) EnterFullscreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Make Youtube app fullscreen")

	const (
		fullscreenBtnID = "com.google.android.youtube:id/fullscreen_button"
	)

	playerView := y.d.Object(androidui.ID(playerViewID))

	startTime := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Wait the player view")
		if err := cuj.FindAndClick(ctx, playerView, uiWaitTime); err != nil {
			return errors.Wrap(err, `failed to find/click the player view: `)
		}

		testing.ContextLog(ctx, "Wait the fullscreen button")
		fsBtn := y.d.Object(androidui.ID(fullscreenBtnID))
		if err := cuj.FindAndClick(ctx, fsBtn, waitTimeAfterClickPlayerView); err != nil {
			return errors.Wrap(err, "failed to find/click the fullscreen button")
		}

		testing.ContextLogf(ctx, "Elapsed time when doing enter fullscreen %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
		return err
	}

	startWaitTime := time.Now()
	// Wait for app window state to fullscreen
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if ws, err := ash.GetARCAppWindowState(ctx, y.tconn, youtubePkg); err != nil {
			return err
		} else if ws != ash.WindowStateFullscreen {
			return errors.New("window state should be fullscreen")
		}
		testing.ContextLogf(ctx, "Elapsed time when waiting app window state to fullscreen %.3f s", time.Since(startWaitTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}); err != nil {
		return err
	}
	return nil
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

	testing.ContextLog(ctx, "Finding player view and click it")
	playerView := y.d.Object(androidui.ID(playerViewID))
	pauseBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(pauseBtnDesc))
	playBtn := y.d.Object(androidui.ID(playPauseBtnID), androidui.Description(playBtnDesc))

	startTime := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		if err := cuj.FindAndClick(ctx, playerView, uiWaitTime); err != nil {
			return errors.Wrap(err, `failed to find/click the player view: `)
		}

		testing.ContextLog(ctx, "Click pause button")
		if err := cuj.FindAndClick(ctx, pauseBtn, waitTimeAfterClickPlayerView); err != nil {
			return errors.Wrap(err, "failed to find/click the pause button")
		}

		testing.ContextLog(ctx, "Verify the video is paused")

		if err := playBtn.WaitForExists(ctx, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the play button")
		}
		// Immediately clicking the target button sometimes doesn't work.
		testing.Sleep(ctx, sleepTime)
		if err := playBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the play button")
		}

		testing.ContextLog(ctx, "Verify the video is playing")
		if err := pauseBtn.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, "failed to find the pause button")
		}

		// Wait time to see the video is playing.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep while video is playing")
		}

		testing.ContextLogf(ctx, "Elapsed time when checking the playback status of youtube app: %.3f s", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed on pauseAndPlayYoutubeApp")
	}
	return nil
}

// Close closes the resources related to video.
func (y *YtApp) Close(ctx context.Context) {
	if y.act != nil {
		y.act.Stop(ctx, y.tconn)
		y.act.Close()
		y.act = nil
	}
}
