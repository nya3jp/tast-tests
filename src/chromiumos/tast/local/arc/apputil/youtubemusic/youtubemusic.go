// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package youtubemusic contains local Tast tests that exercise ytmusic.
package youtubemusic

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
	AppName  = "YT Music"
	pkgName  = "com.google.android.apps.youtube.music"
	idPrefix = "com.google.android.apps.youtube.music:id/"

	searchBtnObjID  = idPrefix + "action_search_button"
	searchTextObjID = idPrefix + "search_edit_text"
	songBtnObjID    = idPrefix + "chip_cloud_chip_text"
	songNameObjID   = idPrefix + "title"
	subtitleObjID   = idPrefix + "subtitle"
	playerObjID     = idPrefix + "player_control_play_pause_replay_button"

	shorUITimeout    = 5 * time.Second
	defaultUITimeout = 30 * time.Second
	longUITimeout    = time.Minute
)

// YouTubeMusic holds resources of ARC app YT Music.
type YouTubeMusic struct {
	*apputil.App

	playingSong string
}

var _ apputil.ARCAudioPlayer = (*YouTubeMusic)(nil)

// New returns YT Music instance.
func New(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*YouTubeMusic, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, AppName, pkgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create arc resource")
	}

	return &YouTubeMusic{App: app}, nil
}

// Song returns the audio information that YouTubeMusic is going to search and play.
func Song(query, subtitle string) *apputil.Audio {
	return &apputil.Audio{Query: query, SubTitle: subtitle}
}

// Play searches the given song and plays it.
func (yt *YouTubeMusic) Play(ctx context.Context, song *apputil.Audio) error {
	if err := yt.SkipPrompts(ctx); err != nil {
		return err
	}

	if err := uiauto.Combine("search a new song to play",
		apputil.FindAndClick(yt.D.Object(ui.ID(searchBtnObjID)), defaultUITimeout),
		apputil.FindAndClick(yt.D.Object(ui.ID(searchTextObjID)), defaultUITimeout),
		yt.Kb.TypeAction(song.Query),
		yt.Kb.AccelAction("Enter"),
		apputil.FindAndClick(yt.D.Object(ui.ID(songBtnObjID), ui.Text("Songs")), defaultUITimeout),
		apputil.FindAndClick(yt.D.Object(ui.ID(subtitleObjID), ui.Text(song.SubTitle)), defaultUITimeout), // Multiple songs with the same title might exist, hence, the subtitle is used.
	)(ctx); err != nil {
		return err
	}

	// Verify YouTubeMusic is playing.
	// Long duration is essential as it is often that low end DUT takes a while to load the audio content to play.
	if exist, err := apputil.IsObjectExists(ctx, yt.D.Object(ui.ID(playerObjID), ui.Description("Pause video")), longUITimeout); err != nil {
		return errors.Wrap(err, "failed to verify YouTubeMusic is playing")
	} else if !exist {
		return errors.Errorf("the YouTube Music is not playing within %s", longUITimeout)
	}
	yt.playingSong = song.Query
	return nil
}

// RemovePlayRecord stops playing and removes the play record to avoid the mini player showing next time the app is launched.
// If the app launches with the mini player appearing, the uiautomator won't be able to be idle and therefore,
// couldn't examine the UI hierarchy and operate on any object.
func (yt *YouTubeMusic) RemovePlayRecord(ctx context.Context) error {
	return uiauto.Combine("stop and remove play record",
		apputil.ClickIfExist(yt.D.Object(ui.ID(playerObjID), ui.Description("Pause video")), defaultUITimeout),
		apputil.SwipeRight(yt.D.Object(ui.ID(songNameObjID), ui.Text(yt.playingSong)), 3),
		apputil.WaitUntilGone(yt.D.Object(ui.ID(playerObjID)), defaultUITimeout),
	)(ctx)
}

// Pause stops youtube music.
func (yt *YouTubeMusic) Pause(ctx context.Context) error {
	if err := apputil.FindAndClick(yt.D.Object(ui.ID(playerObjID), ui.Description("Pause video")), defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to pause")
	}

	if exist, err := apputil.IsObjectExists(ctx, yt.D.Object(ui.ID(playerObjID), ui.Description("Play video")), defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to verify YouTubeMusic is paused")
	} else if !exist {
		return errors.Errorf("the YouTube Music is not paused within %s", defaultUITimeout)
	}
	return nil
}

// Resume resumes youtube music.
func (yt *YouTubeMusic) Resume(ctx context.Context) error {
	if err := apputil.FindAndClick(yt.D.Object(ui.ID(playerObjID), ui.Description("Play video")), defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to resume")
	}

	// Long duration is essential as it is often that low end DUT takes a while to load the audio content to play.
	if exist, err := apputil.IsObjectExists(ctx, yt.D.Object(ui.ID(playerObjID), ui.Description("Pause video")), longUITimeout); err != nil {
		return errors.Wrap(err, "failed to verify YouTubeMusic is playing")
	} else if !exist {
		return errors.Errorf("the YouTube Music is not resumed within %s", longUITimeout)
	}
	return nil
}

// SkipPrompts skips multiple prompts.
// Click the button to close any redundant windows that appear, but we don't need to stop the test if no window appears.
func (yt *YouTubeMusic) SkipPrompts(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing prompts")

	if err := apputil.DismissMobilePrompt(ctx, yt.Tconn); err != nil {
		return errors.Wrap(err, `failed to dismiss "This app is designed for mobile" prompt`)
	}

	prompts := []struct {
		obj     *ui.Object
		name    string
		cleared bool
	}{
		{yt.D.Object(ui.Text("DISMISS")), "DISMISS", false},
		{yt.D.Object(ui.TextStartsWith("SKIP")), "SKIP", false},
		{yt.D.Object(ui.DescriptionStartsWith("SKIP")), "SKIP", false},
		{yt.D.Object(ui.Text("NO, THANKS")), "NO, THANKS", false},
		{yt.D.Object(ui.Text("NO THANKS")), "NO THANKS", false},
		{yt.D.Object(ui.Description("Close")), "Close", false},
	}

	// The occuring of the prompts is random. Instead of waiting a longer time for each
	// prompt, we repeatedly check the prompts with short wait time but high frequency.
	// The total check time is controlled under 30 seconds.
	totalClearTime := 30 * time.Second
	totalCleared := 0
	timeout := 2 * time.Second
	clearFail := false // Indicate if there are UI error when clearing prompts.
	err := testing.Poll(ctx, func(c context.Context) error {
		for _, prompt := range prompts {
			if prompt.cleared {
				continue
			}

			if err := prompt.obj.WaitForExists(ctx, timeout); err != nil {
				if ui.IsTimeout(err) {
					continue
				}
				clearFail = true
				return testing.PollBreak(errors.Wrap(err, "failed to wait for the target object"))
			}
			if err := prompt.obj.Click(ctx); err != nil {
				clearFail = true
				return testing.PollBreak(errors.Wrap(err, "failed to click ui object to clear prompts"))
			}

			prompt.cleared = true
			totalCleared++
			testing.ContextLogf(ctx, "Prompt %q has been cleared", prompt.name)
		}

		if totalCleared >= len(prompts) {
			return nil
		}
		return errors.New("not all prompts have been cleared")
	}, &testing.PollOptions{Timeout: totalClearTime, Interval: time.Second})

	testing.ContextLogf(ctx, "Total %d prompt(s) have been cleared", totalCleared)

	if err != nil && clearFail {
		return errors.Wrap(err, "failed to clear prompts")
	}
	// All prompts have been cleared, or timed out to wait for prompts to occur.
	return nil
}
