// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package youtube contains local Tast tests that exercise youtube.
package youtube

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// AppName is the name of ARC app.
	AppName = "YouTube"
	// PkgName is the package name of ARC app.
	PkgName = "com.google.android.youtube"

	idPrefix = PkgName + ":id/"

	shortTimeout     = 2 * time.Second
	defaultUITimeout = 15 * time.Second
)

// Youtube holds resources of ARC app Youtube.
type Youtube struct {
	*apputil.App

	// isPremium denotes whether a premium account is being used.
	isPremium bool
}

var _ apputil.ARCMediaPlayer = (*Youtube)(nil)

// NewApp returns Youtube instance.
func NewApp(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*Youtube, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, AppName, PkgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create arc resource")
	}

	return &Youtube{App: app}, nil
}

// ClearPrompts clears the start up prompts of Youtube app.
func (yt *Youtube) ClearPrompts(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing prompts")

	if err := apputil.DismissMobilePrompt(ctx, yt.Tconn); err != nil {
		return errors.Wrap(err, "failed to dismiss 'This app is designed for mobile' prompt")
	}

	dismissID := idPrefix + "dismiss"
	skipTrial := yt.Device.Object(ui.ID(dismissID), ui.Text("SKIP TRIAL"))
	closePrompt := yt.Device.Object(ui.Description("Close"), ui.ClassName("android.view.ViewGroup"))
	noThanksEle := yt.Device.Object(ui.ID(dismissID), ui.Text("NO THANKS"))
	if err := uiauto.Combine("clear prompts",
		apputil.ClickIfExist(skipTrial, shortTimeout),
		apputil.ClickIfExist(closePrompt, shortTimeout),
		apputil.ClickIfExist(noThanksEle, shortTimeout),
	)(ctx); err != nil {
		return err
	}

	premiumLogo := yt.Device.Object(ui.Description("YouTube Premium"))
	if isExist, err := apputil.CheckObjectExists(ctx, premiumLogo, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to check premium logo")
	} else if isExist {
		yt.isPremium = true
		testing.ContextLog(ctx, "Current account is a premium account")
	}

	return nil
}

// Play opens and plays a video on youtube app.
func (yt *Youtube) Play(ctx context.Context, media *apputil.Media) error {
	if err := yt.ClearPrompts(ctx); err != nil {
		return errors.Wrap(err, "failed to clear prompts")
	}

	testing.ContextLog(ctx, "Search and play video")

	searchButton := yt.Device.Object(ui.ID(idPrefix + "menu_item_1"))
	searchEditText := yt.Device.Object(ui.ID(idPrefix + "search_edit_text"))
	searchLayoutView := yt.Device.Object(ui.ID(idPrefix + "search_layout"))

	if err := uiauto.Combine("search video by url",
		apputil.FindAndClick(searchButton, defaultUITimeout),
		apputil.FindAndClick(searchEditText, defaultUITimeout),
		yt.KB.TypeAction(media.Query),
		// Sending keyboard event via ChromeOS can be influenced by pop-up views in ARC while the Android key event KEYCODE_ENTER won't have such issue.
		func(ctx context.Context) error { return yt.Device.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0) },
		apputil.WaitUntilGone(searchLayoutView, shortTimeout),
	)(ctx); err != nil {
		return err
	}

	resultVideo := yt.Device.Object(ui.DescriptionContains(media.Subtitle))
	startTime := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := apputil.FindAndClick(resultVideo, defaultUITimeout)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find %q", media.Subtitle)
		}

		testing.ContextLog(ctx, "Elapsed time when waiting the video list: ", time.Since(startTime).Seconds())
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to click %q", media.Subtitle)
	}

	// Skip AD if exist.
	if !yt.isPremium {
		skipAds := yt.Device.Object(ui.ID(idPrefix + "skip_ad_button"))
		if err := apputil.ClickIfExist(skipAds, defaultUITimeout)(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Skip Ads'")
		}
	}

	return yt.startFromBeginning(ctx)
}

// startFromBeginning starts the video to the beginning by dragging the current time node to the most left of the bar.
func (yt *Youtube) startFromBeginning(ctx context.Context) error {
	timeBarObj := yt.Device.Object(ui.ClassName("android.widget.SeekBar"))

	timestampsRect, err := timeBarObj.GetBounds(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get node bounds")
	}

	return apputil.DragAndDrop(yt.ARC, timestampsRect.CenterPoint(), timestampsRect.LeftCenter(), time.Second)(ctx)
}

// GetYoutubePlayingTime returns the current time of video.
func (yt *Youtube) GetYoutubePlayingTime(ctx context.Context) (string, error) {
	testing.ContextLog(ctx, "Get youtube playing time")
	watchPlayerID := idPrefix + "watch_player"
	timebarCurrentTimeID := idPrefix + "time_bar_current_time"

	var playTime string
	var err error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		playerView := yt.Device.Object(ui.ID(watchPlayerID))
		playtimeNode := yt.Device.Object(ui.ID(timebarCurrentTimeID))

		if err := uiauto.NamedCombine("clicking the player view to find the time bar",
			playerView.Click,
			apputil.WaitForExists(playtimeNode, 5*time.Second),
		)(ctx); err != nil {
			return err
		}

		playTime, err = playtimeNode.GetText(ctx)
		if err != nil {
			return err
		}

		testing.ContextLogf(ctx, "Youtube playing time is %s", playTime)
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: 5 * time.Second}); err != nil {
		return "", err
	}

	return playTime, nil
}

// IsPlaying checks if youtube app is playing video.
func (yt *Youtube) IsPlaying(ctx context.Context) (bool, error) {
	timeStart, err := yt.GetYoutubePlayingTime(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get video time")
	}

	// Wait for a while to verify playing by checking time difference on progress bar.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return false, errors.Wrap(err, "failed to sleep")
	}

	timeEnd, err := yt.GetYoutubePlayingTime(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get video time")
	}

	return timeStart != timeEnd, nil
}
