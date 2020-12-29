// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package everydaymultitaskingcuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	spotifyPackageName = "com.spotify.music"
	spotifyIDPrefix    = "com.spotify.music:id/"
)

type spotify struct {
	d          *androidui.Device
	account    string
	waitTime   time.Duration
	firstLogin bool
}

func newSpotify(d *androidui.Device, account string, timeout time.Duration) *spotify {
	return &spotify{
		d:        d,
		account:  account,
		waitTime: waitTime,
	}
}

func (s *spotify) play(ctx context.Context) error {
	const playPauseButtonID = spotifyIDPrefix + "play_pause_button"

	testing.ContextLog(ctx, "Signing into Spotify")
	signIn := s.d.Object(androidui.Text("Continue with Google"))
	if err := signIn.WaitForExists(ctx, waitTime); err != nil {
		testing.ContextLog(ctx, `"Continue with Google" button not found, assuming splash screen has been dismissed already`)
	} else if err := signIn.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click "Continue with Google" button`)
	} else {
		accountButton := s.d.Object(androidui.Text(s.account))
		if err := cuj.FindAndClickAction(accountButton, s.waitTime)(ctx); err != nil {
			testing.ContextLog(ctx, `The button "account button" not found, sign in directly`)
		}
		s.firstLogin = true
	}

	testing.ContextLog(ctx, "Clearing prompts")
	dismiss := s.d.Object(androidui.Text("DISMISS"))
	promp := s.d.Object(androidui.Text("NO, THANKS"))
	if err := uiauto.Combine("clear prompt",
		cuj.ClickIfExistAction(dismiss, waitTime),
		cuj.ClickIfExistAction(promp, waitTime),
	)(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Try to play a song")
	playButton := s.d.Object(androidui.ID(playPauseButtonID), androidui.Enabled(true), androidui.Description("Play"))

	if err := playButton.WaitForExists(ctx, s.waitTime); err != nil {
		// If Spotify is installed very first time, there will be no last listened songs.
		// Search a song to play.
		if err := s.searchSongAndPlay(ctx); err != nil {
			return errors.Wrap(err, "failed to search song and play")
		}
	} else {
		// Otherwise, can play a song directly by clicking the play button,
		// which will play the last listened song.
		if err := s.playLastListenedSong(ctx, playButton); err != nil {
			return errors.Wrap(err, "failed to play last listened song")
		}
	}

	// Verify Soptify is playing.
	pauseButton := s.d.Object(androidui.ID(playPauseButtonID), androidui.Enabled(true), androidui.Description("Pause"))
	if err := pauseButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait for pause button to exist, Spotify is not playing")
	}

	testing.ContextLog(ctx, "Spotify is playing")
	return nil
}

func (s *spotify) playLastListenedSong(ctx context.Context, playButton *androidui.Object) error {
	testing.ContextLog(ctx, "Try to play last listened song")

	if err := cuj.FindAndClickAction(playButton, s.waitTime)(ctx); err != nil {
		testing.ContextLog(ctx, `Failed to play last listened song, try to search a song and play`)
		return s.searchSongAndPlay(ctx)
	}

	return nil
}

func (s *spotify) searchSongAndPlay(ctx context.Context) error {
	const (
		searchTabID   = spotifyIDPrefix + "search_tab"
		searchFieldID = spotifyIDPrefix + "find_search_field_text"
		queryID       = spotifyIDPrefix + "query"
		childrenID    = spotifyIDPrefix + "children"
		albumName     = "Photograph"
		singerName    = "Song • Ed Sheeran"
	)

	var (
		searchTab    = s.d.Object(androidui.ID(searchTabID))
		searchField  = s.d.Object(androidui.ID(searchFieldID))
		query        = s.d.Object(androidui.ID(queryID))
		singerButton = s.d.Object(androidui.Text(singerName))
	)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()
	testing.ContextLog(ctx, "Try to search a song and play")
	if err := uiauto.Combine("search song",
		cuj.FindAndClickAction(searchTab, defaultUITimeout),
		cuj.FindAndClickAction(searchField, defaultUITimeout),
		cuj.FindAndClickAction(query, defaultUITimeout),
		kb.TypeAction(albumName),
		cuj.FindAndClickAction(singerButton, defaultUITimeout),
	)(ctx); err != nil {
		return err
	}

	var shufflePlayButton *androidui.Object
	if s.firstLogin {
		shufflePlayButton = s.d.Object(androidui.Text("SHUFFLE PLAY"))
	} else {
		shufflePlayButton = s.d.Object(androidui.ID(childrenID), androidui.ClassName("android.widget.LinearLayout"))
	}

	// It might automatically start playing after click singerButton,
	// so skip if shufflePlayButton not found.
	if err := cuj.ClickIfExistAction(shufflePlayButton, defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, `failed to click "shuffle play button"`)
	}

	return nil
}
