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

func openAndPlayYoutubeApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, video Video, index int) (*arc.Activity, error) {
	const (
		youtubePkg              = "com.google.android.youtube"
		youtubeAct              = "com.google.android.apps.youtube.app.WatchWhileActivity"
		accountImageID          = "com.google.android.youtube:id/image"
		searchButtonID          = "com.google.android.youtube:id/menu_item_1"
		searchEditTextID        = "com.google.android.youtube:id/search_edit_text"
		playerViewID            = "com.google.android.youtube:id/player_view"
		accountImageDescription = "Account"
		timeout                 = time.Second * 15
	)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup ARC and Play Store")
	}
	defer d.Close(ctx)

	testing.ContextLog(ctx, "Open Youtube app")
	act, err := arc.NewActivity(a, youtubePkg, youtubeAct)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the YouTube activity")
	}

	if err := act.Start(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to start the YouTube app")
	}

	testing.ContextLog(ctx, "Wait for YouTube app launched")
	accountImage := d.Object(androidui.ID(accountImageID), androidui.DescriptionContains(accountImageDescription))
	if err := accountImage.WaitForExists(ctx, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to check for Youtube app launched")
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
		if err := searchEditText.WaitForExists(ctx, timeout); err != nil {
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
			if err := firstVideo.WaitForExists(ctx, timeout); err != nil {
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
		errCount := 0
		testing.ContextLog(ctx, "Switch Quality")
		if err := testing.Poll(ctx, func(context.Context) error {
			testing.ContextLog(ctx, "Find player view and click it")
			playerView := d.Object(androidui.ID(playerViewID))
			if err := playerView.WaitForExists(ctx, timeout); err != nil {
				return errors.Wrap(err, "failed to find the player view on switch quality")
			}
			if err := playerView.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click the player view on switch quality")
			}

			testing.ContextLog(ctx, "Find more button and click it")
			moreBtn := d.Object(androidui.Description("More options"))
			if err := moreBtn.WaitForExists(ctx, timeout); err != nil {
				return errors.Wrap(err, `failed to find the 'More options'`)
			}
			if err := moreBtn.Click(ctx); err != nil {
				return errors.Wrap(err, `failed to click 'More options'`)
			}

			testing.ContextLog(ctx, "Find the Quality button")
			qualityBtn := d.Object(androidui.Text("Quality"))
			if err := qualityBtn.WaitForExists(ctx, timeout); err != nil {
				return errors.Wrap(err, "failed to find the Quality")
			}
			if err := qualityBtn.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click Quality")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
			return err
		}

		if err := testing.Poll(ctx, func(context.Context) error {
			if errCount > 0 {
				errCount = 0
				if index == 1 {
					testing.ContextLog(ctx, "change 1080p60 to 720p60")
					resolution = "720p60"
				} else if index == 2 {
					testing.ContextLog(ctx, "change 1080p to 720p")
					resolution = "720p"
				}
			}
			testing.ContextLog(ctx, "Find the target quality")
			targetQuality := d.Object(androidui.Text(resolution))
			if err := targetQuality.WaitForExists(ctx, 2*time.Second); err != nil {
				errCount++
				testing.ContextLogf(ctx, "Failed to find the target quality, index: %d, errCount: %d", index, errCount)
				return errors.Wrap(err, "failed to find the target quality")
			}
			if err := targetQuality.Click(ctx); err != nil {
				return errors.Wrap(err, "failed to click target quality")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second, Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to find and click target quality")
		}
		return nil
	}

	if err := playVideo(); err != nil {
		return nil, errors.Wrap(err, "failed to play video")
	}

	if err := switchQuality(video.quality); err != nil {
		return nil, errors.Wrap(err, "failed to switch Quality")
	}

	return act, nil
}

func checkYoutubeAppPIP(ctx context.Context, tconn *chrome.TestConn) error {
	const youtubePkg = "com.google.android.youtube"
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

func enterYoutubeAppFullscreen(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	const (
		youtubePkg      = "com.google.android.youtube"
		playerViewID    = "com.google.android.youtube:id/player_view"
		fullscreenBtnID = "com.google.android.youtube:id/fullscreen_button"
		timeout         = time.Second * 15
	)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup ARC and Play Store")
	}
	defer d.Close(ctx)

	playerView := d.Object(androidui.ID(playerViewID))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		testing.ContextLog(ctx, "Wait the player view")
		if err := playerView.WaitForExists(ctx, timeout); err != nil {
			return errors.Wrap(err, `failed to find the player view`)
		}
		testing.ContextLog(ctx, "Click the player view")
		if err := playerView.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the player view")
		}

		fsBtn := d.Object(androidui.ID(fullscreenBtnID))
		testing.ContextLog(ctx, "Wait the fullscreen button")
		if err := fsBtn.WaitForExists(ctx, timeout); err != nil {
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

func pauseAndPlayYoutubeApp(ctx context.Context, a *arc.ARC) error {
	const (
		playerViewID   = "com.google.android.youtube:id/player_view"
		playPauseBtnID = "com.google.android.youtube:id/player_control_play_pause_replay_button"
		playBtnDesc    = "Play video"
		pauseBtnDesc   = "Pause video"
		timeout        = time.Second * 15
		waitTime       = time.Second * 3
	)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup ARC and Play Store")
	}
	defer d.Close(ctx)

	testing.ContextLog(ctx, "Finding player view and click it")
	playerView := d.Object(androidui.ID(playerViewID))
	pauseBtn := d.Object(androidui.ID(playPauseBtnID), androidui.Description(pauseBtnDesc))
	playBtn := d.Object(androidui.ID(playPauseBtnID), androidui.Description(playBtnDesc))

	if err := playerView.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to find the player view")
	}
	if err := playerView.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the player view")
	}

	testing.ContextLog(ctx, "Verify the video is playing")
	if err := pauseBtn.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to find the pause button")
	}

	testing.ContextLog(ctx, "Click pause button")
	if err := pauseBtn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the pause button")
	}

	testing.ContextLog(ctx, "Verify the video is paused")
	if err := playBtn.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to find the play button")
	}

	// Wait time to see the video is paused
	testing.Sleep(ctx, waitTime)

	testing.ContextLog(ctx, "Click play button")
	if err := playBtn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the play button")
	}

	testing.ContextLog(ctx, "Verify the video is playing")
	if err := pauseBtn.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to find the pause button")
	}

	// Wait time to see the video is paused
	testing.Sleep(ctx, waitTime)
	return nil
}
