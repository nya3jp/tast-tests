// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	youtubePkg                   = "com.google.android.youtube"
	playerViewID                 = "com.google.android.youtube:id/player_view"
	uiWaitTime                   = time.Second * 15
	waitTimeAfterClickPlayerView = time.Second * 3
)

func openAndPlayYoutubeApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *androidui.Device, video Video, index int) (*arc.Activity, int64, error) {
	const (
		youtubeAct              = "com.google.android.apps.youtube.app.WatchWhileActivity"
		accountImageID          = "com.google.android.youtube:id/image"
		searchButtonID          = "com.google.android.youtube:id/menu_item_1"
		searchEditTextID        = "com.google.android.youtube:id/search_edit_text"
		moreOptions             = "com.google.android.youtube:id/player_overflow_button"
		noThanksID              = "com.google.android.youtube:id/dismiss"
		accountImageDescription = "Account"
	)

	testing.ContextLog(ctx, "Open Youtube app")
	startTime := time.Now()
	act, err := arc.NewActivity(a, youtubePkg, youtubeAct)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to create the YouTube activity")
	}

	if err := act.Start(ctx, tconn); err != nil {
		return nil, 0, errors.Wrap(err, "failed to start the YouTube app")
	}
	appStartTime := time.Now().Sub(startTime).Milliseconds()

	testing.ContextLog(ctx, "Wait for YouTube app launched")
	accountImage := d.Object(androidui.ID(accountImageID), androidui.DescriptionContains(accountImageDescription))
	if err := accountImage.WaitForExists(ctx, uiWaitTime); err != nil {
		return nil, appStartTime, errors.Wrap(err, "failed to check for Youtube app launched")
	}

	// Skip "NO THANKS" if it shows up.
	noThanksEle := d.Object(androidui.ID(noThanksID), androidui.Text("NO THANKS"))
	if err := noThanksEle.WaitForExists(ctx, time.Second*5); err == nil {
		noThanksEle.Click(ctx)
	}

	playVideo := func() error {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to open the keyboard")
		}
		defer kb.Close()

		searchButton := d.Object(androidui.ID(searchButtonID))
		if err := searchButton.Click(ctx); err != nil {
			return err
		}

		testing.ContextLog(ctx, `Find 'searchTextField'`)
		searchEditText := d.Object(androidui.ID(searchEditTextID))
		if err := searchEditText.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, `failed to find 'searchTextfield'`)
		}
		if err := searchEditText.Click(ctx); err != nil {
			return err
		}

		testing.ContextLog(ctx, "Typing video url")
		if err := kb.Type(ctx, video.url); err != nil {
			return err
		}
		if err := kb.Accel(ctx, "enter"); err != nil {
			return errors.Wrap(err, "failed to type the enter key")
		}

		firstVideo := d.Object(androidui.ClassName("android.view.ViewGroup"), androidui.Index(1))
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			testing.ContextLog(ctx, "Wait the video list")
			if err := firstVideo.WaitForExists(ctx, uiWaitTime); err != nil {
				return errors.Wrap(err, `failed to find 'First Video'`)
			}
			testing.ContextLog(ctx, "Click the first video")
			if err := firstVideo.Click(ctx); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 30 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to click first video")
		}
		return nil
	}

	switchQuality := func(resolution string) error {
		testing.ContextLog(ctx, "Switch Quality")
		if err := testing.Poll(ctx, func(context.Context) error {
			testing.ContextLog(ctx, "Find player view and click it")
			playerView := d.Object(androidui.ID(playerViewID))
			if err := playerView.WaitForExists(ctx, uiWaitTime); err != nil {
				return errors.Wrap(err, "failed to find the player view on switch quality")
			}
			if err := playerView.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click the player view on switch quality")
			}

			testing.ContextLog(ctx, "Find more button and click it")
			moreBtn := d.Object(androidui.ID(moreOptions))
			if err := moreBtn.WaitForExists(ctx, waitTimeAfterClickPlayerView); err != nil {
				return errors.Wrap(err, `failed to find the 'More options'`)
			}
			if err := moreBtn.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click 'More options'`)
			}

			testing.ContextLog(ctx, "Find the Quality button")
			qualityBtn := d.Object(androidui.Text("Quality"))
			if err := qualityBtn.WaitForExists(ctx, uiWaitTime); err != nil {
				return errors.Wrap(err, "failed to find the Quality")
			}
			if err := qualityBtn.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click Quality")
			}
			testing.ContextLog(ctx, "Find the target quality")
			targetQuality := d.Object(androidui.Text(resolution))
			if err := targetQuality.WaitForExists(ctx, 2*time.Second); err != nil {
				testing.ContextLogf(ctx, "Failed to find the target quality: %s", resolution)
				return errors.Wrap(err, "failed to find the target quality")
			}
			if err := targetQuality.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click target quality")
			}
			testing.ContextLogf(ctx, "Clicked target quality: %s", resolution)
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			return err
		}
		return nil
	}

	if err := playVideo(); err != nil {
		return nil, appStartTime, errors.Wrap(err, "failed to play video")
	}

	if err := switchQuality(video.quality); err != nil {
		return nil, appStartTime, errors.Wrap(err, "failed to switch Quality")
	}

	return act, appStartTime, nil
}

func checkYoutubeAppPIP(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Check window state should be PIP")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if ws, err := ash.GetARCAppWindowState(ctx, tconn, youtubePkg); err != nil {
			return errors.Wrap(err, "can not get ARC App Window State: ")
		} else if ws != ash.WindowStatePIP {
			testing.ContextLog(ctx, "Press alt + '='")
			if err := kb.AccelPress(ctx, "Alt"); err != nil {
				return errors.Wrap(err, "failed to press alt")
			}
			defer kb.AccelRelease(ctx, "Alt")
			if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			if err := kb.Accel(ctx, "="); err != nil {
				return errors.Wrap(err, `failed to type '='`)
			}
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to wait")
			}
			return errors.Wrap(err, "window state should be PIP: ")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
		return err
	}
	return nil
}

func enterYoutubeAppFullscreen(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *androidui.Device) error {
	const (
		fullscreenBtnID = "com.google.android.youtube:id/fullscreen_button"
	)

	playerView := d.Object(androidui.ID(playerViewID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Wait the player view")
		if err := playerView.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, `failed to find the player view`)
		}
		testing.ContextLog(ctx, "Click the player view")
		if err := playerView.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the player view")
		}

		fsBtn := d.Object(androidui.ID(fullscreenBtnID))
		testing.ContextLog(ctx, "Wait the fullscreen button")
		if err := fsBtn.WaitForExists(ctx, waitTimeAfterClickPlayerView); err != nil {
			return errors.Wrap(err, `failed to find the fullscreen button`)
		}
		testing.ContextLog(ctx, "Click the fullscreen button")
		if err := fsBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the fullscreen button")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
		return err
	}

	// Wait for app window state to fullscreen
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if ws, err := ash.GetARCAppWindowState(ctx, tconn, youtubePkg); err != nil {
			return err
		} else if ws != ash.WindowStateFullscreen {
			return errors.New("window state should be fullscreen")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}); err != nil {
		return err
	}
	return nil
}

func pauseAndPlayYoutubeApp(ctx context.Context, a *arc.ARC, d *androidui.Device) error {
	const (
		playPauseBtnID = "com.google.android.youtube:id/player_control_play_pause_replay_button"
		playBtnDesc    = "Play video"
		pauseBtnDesc   = "Pause video"
		sleepTime      = time.Second * 3
	)

	testing.ContextLog(ctx, "Finding player view and click it")
	playerView := d.Object(androidui.ID(playerViewID))
	pauseBtn := d.Object(androidui.ID(playPauseBtnID), androidui.Description(pauseBtnDesc))
	playBtn := d.Object(androidui.ID(playPauseBtnID), androidui.Description(playBtnDesc))

	if err := testing.Poll(ctx, func(ctx context.Context) error {

		if err := playerView.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, "failed to find the player view")
		}
		if err := playerView.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the player view")
		}

		testing.ContextLog(ctx, "Verify the video is playing")
		if err := pauseBtn.WaitForExists(ctx, waitTimeAfterClickPlayerView); err != nil {
			return errors.Wrap(err, "failed to find the pause button")
		}

		testing.ContextLog(ctx, "Click pause button")
		if err := pauseBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the pause button")
		}

		testing.ContextLog(ctx, "Verify the video is paused")
		if err := playBtn.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, "failed to find the play button")
		}

		// Pause the video for a while.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep while video is paused")
		}

		testing.ContextLog(ctx, "Click play button")
		if err := playBtn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the play button")
		}

		testing.ContextLog(ctx, "Verify the video is playing")
		if err := pauseBtn.WaitForExists(ctx, uiWaitTime); err != nil {
			return errors.Wrap(err, "failed to find the pause button")
		}

		// Play the video for a while.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "failed to sleep while video is playing")
		}

		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed on pauseAndPlayYoutubeApp: ")
	}
	return nil
}
