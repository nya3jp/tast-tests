// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package spotify contains local Tast tests that exercise spotify.
package spotify

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
	AppName = "Spotify"
	// PackageName is the package name of ARC app.
	PackageName = "com.spotify.music"

	spotifyIDPrefix = "com.spotify.music:id/"
	searchTabID     = spotifyIDPrefix + "search_tab"

	adTimeout        = 2 * time.Minute  // Used to wait for advertisements.
	mediumUITimeout  = 30 * time.Second // Used for situations where UI response are slower.
	defaultUITimeout = 15 * time.Second // Used for situations where UI response might be slow.
	shortUITimeout   = 5 * time.Second  // Used for situations where UI response are faster.
)

// Spotify holds the information used to do Spotify APP testing.
type Spotify struct {
	*apputil.App

	account    string
	firstLogin bool
}

var _ apputil.ARCAudioPlayer = (*Spotify)(nil)

// New returns the the manager of Spotify, caller will able to control Spotify app through this object.
func New(ctx context.Context, kb *input.KeyboardEventWriter, a *arc.ARC, tconn *chrome.TestConn, account string) (*Spotify, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, AppName, PackageName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new ARC UI device")
	}
	return &Spotify{
		App:     app,
		account: account,
	}, nil
}

// Song returns the audio information that Spotify is going to search and play.
func Song(album, artist string) *apputil.Audio {
	return &apputil.Audio{Album: album, Artist: artist}
}

// Play plays a song.
func (s *Spotify) Play(ctx context.Context, song *apputil.Audio) error {
	const playPauseButtonID = spotifyIDPrefix + "play_pause_button"
	playButton := s.D.Object(ui.ID(playPauseButtonID), ui.Enabled(true), ui.Description("Play"))

	// If it has been played, it can play the song directly.
	if err := apputil.FindAndClick(playButton, shortUITimeout)(ctx); err == nil {
		testing.ContextLog(ctx, "Play Spotify directly")
	} else {
		if exist, err := apputil.IsObjectExists(ctx, s.D.Object(ui.ID(searchTabID)), defaultUITimeout); err != nil {
			return errors.Wrap(err, "failed to check is search tab exist")
		} else if !exist {
			if err := s.loginIfRequired(ctx); err != nil {
				return errors.Wrap(err, "failed to login into Spotify")
			}
			if err := s.clearPrompts(ctx); err != nil {
				return err
			}
			if err := s.waitUntilHomePageShows(ctx); err != nil {
				return errors.Wrap(err, "failed to wait until home page shows")
			}
		}
		if err := s.searchSongAndPlay(ctx, song); err != nil {
			return errors.Wrap(err, "failed to search song and play")
		}
	}

	testing.ContextLog(ctx, "Verify that Spotify is playing")
	pauseButton := s.D.Object(ui.ID(playPauseButtonID), ui.Enabled(true), ui.Description("Pause"))
	// Sometimes there will be advertisements that need to be verified at a later time,
	// so use adTimeout here.
	if err := pauseButton.WaitForExists(ctx, adTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for pause button to exist, Spotify is not playing")
	}

	testing.ContextLog(ctx, "Spotify is playing")
	return nil
}

// loginIfRequired logins to Spotify if it is not logged in.
func (s *Spotify) loginIfRequired(ctx context.Context) error {
	// The "This app is designed for mobile" prompt needs to be dismissed to get to the log in page.
	if err := apputil.DismissMobilePrompt(ctx, s.Tconn); err != nil {
		return errors.Wrap(err, `failed to dismiss "This app is designed for mobile" prompt`)
	}

	// Spotify is performing A/B testing and there are two possible UI results,
	// 1. A label with text "Already have an account? Log in."
	// 2. A button with text "Log in"
	if exist, err := apputil.IsObjectExists(ctx, s.D.Object(ui.TextContains("Log in")), shortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find login button")
	} else if !exist {
		testing.ContextLog(ctx, "Already signed in to Spotify")
		return nil
	}

	testing.ContextLog(ctx, "Signing into Spotify")

	// Spotify is performing A/B testing and there are two possible UI results,
	// one is login buttons with text, another is login buttons with description.
	signInWithGoogleBtns := map[*ui.Object]string{
		s.D.Object(ui.Text("Continue with Google")):        "sign in button with text",
		s.D.Object(ui.Description("Continue with Google")): "sign in button with description",
	}
	if err := apputil.ClickAnyFromObjectPool(ctx, signInWithGoogleBtns, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to login with Google account")
	}

	// The account selection dialog does not always appear, assuming signed in already if the dialog didn't appear.
	if err := apputil.ClickIfExist(s.D.Object(ui.Text(s.account)), defaultUITimeout)(ctx); err != nil {
		return errors.Wrapf(err, "failed to login with %q", s.account)
	}
	s.firstLogin = true

	return nil
}

func (s *Spotify) clearPrompts(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing prompts")

	prompts := []struct {
		obj     *ui.Object
		name    string
		cleared bool
	}{
		{s.D.Object(ui.Text("DISMISS")), "DISMISS", false},
		{s.D.Object(ui.Text("NO, THANKS")), "NO, THANKS", false},
		{s.D.Object(ui.ID(spotifyIDPrefix + "app_rater_dialog_button_dismiss")), "FREE TRIAL", false},
	}

	// The occuring of the prompts is random. Instead of waiting a longer time for each
	// prompt, we repeatedly check the prompts with short wait time but high frequency.
	// The total check time is controlled under 30 seconds.
	totalClearTime := 30 * time.Second
	totalCleared := 0
	clearFail := false // Indicate if there are UI error when clearing prompts.
	err := testing.Poll(ctx, func(c context.Context) error {
		for _, prompt := range prompts {
			if prompt.cleared {
				continue
			}

			if err := prompt.obj.WaitForExists(ctx, 2*time.Second); err != nil {
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
	// All prompts have been cleared, or timed out to wait for prompts to occure.
	return nil
}

func (s *Spotify) waitUntilHomePageShows(ctx context.Context) error {
	// It's the close button in the player overlay view.
	playerOverlayViewCloseBtn := s.D.Object(ui.ID(spotifyIDPrefix+"close_button"), ui.Description("Close"))
	// An overlay view might automatically show, need to dismiss it to continue tests.
	if err := apputil.ClickIfExist(playerOverlayViewCloseBtn, defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to close the overlay view")
	}

	if err := s.D.Object(ui.ID(searchTabID)).WaitForExists(ctx, mediumUITimeout); err != nil {
		return errors.Wrapf(err, `failed to wait for search tab to exist in %v`, mediumUITimeout)
	}

	return nil
}

// searchSongAndPlay searches a song and play.
// If Spotify is installed for the first time, there will be no last listened song.
// Search for a song to play.
func (s *Spotify) searchSongAndPlay(ctx context.Context, song *apputil.Audio) error {
	const (
		searchFieldID = spotifyIDPrefix + "find_search_field_text"
		queryID       = spotifyIDPrefix + "query"
		childrenID    = spotifyIDPrefix + "children"
	)

	var (
		searchTab    = s.D.Object(ui.ID(searchTabID))
		searchField  = s.D.Object(ui.ID(searchFieldID))
		query        = s.D.Object(ui.ID(queryID))
		singerButton = s.D.Object(ui.Text(song.Artist))
	)

	testing.ContextLog(ctx, "Try to search a song and play")

	if err := uiauto.Combine("search song",
		apputil.FindAndClick(searchTab, defaultUITimeout),
		apputil.FindAndClick(searchField, defaultUITimeout),
		apputil.FindAndClick(query, defaultUITimeout),
		s.Kb.TypeAction(song.Album),
		apputil.FindAndClick(singerButton, defaultUITimeout),
	)(ctx); err != nil {
		return err
	}

	var shufflePlayButton *ui.Object
	if s.firstLogin {
		shufflePlayButton = s.D.Object(ui.Text("SHUFFLE PLAY"))
	} else {
		shufflePlayButton = s.D.Object(ui.ID(childrenID), ui.ClassName("android.widget.LinearLayout"))
	}

	// It might automatically start playing after click singerButton,
	// so skip if shufflePlayButton not found.
	if err := apputil.ClickIfExist(shufflePlayButton, defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, `failed to click "shuffle play button"`)
	}

	return nil
}
