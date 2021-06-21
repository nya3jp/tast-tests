// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package everydaymultitaskingcuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	spotifyPackageName = "com.spotify.music"
	spotifyIDPrefix    = "com.spotify.music:id/"
	defaultUITimeout   = 30 * time.Second
)

// Spotify holds the information used to do Spotify APP testing.
type Spotify struct {
	kb         *input.KeyboardEventWriter
	a          *arc.ARC
	d          *ui.Device
	account    string
	uiTimeout  time.Duration
	firstLogin bool
}

// NewSpotify returns the the manager of Spotify, caller will able to control Spotify app through this object.
func NewSpotify(kb *input.KeyboardEventWriter, a *arc.ARC, d *ui.Device, account string, timeout time.Duration) *Spotify {
	return &Spotify{
		kb:        kb,
		a:         a,
		d:         d,
		account:   account,
		uiTimeout: timeout,
	}
}

// Install installs Soptify app.
func (s *Spotify) Install(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	installCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := playstore.InstallApp(installCtx, s.a, s.d, spotifyPackageName, -1); err != nil {
		return errors.Wrapf(err, "failed to install %s", spotifyPackageName)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}
	return nil
}

// Launch launches Soptify app.
func (s *Spotify) Launch(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) (time.Duration, error) {
	if w, err := ash.GetARCAppWindowInfo(ctx, tconn, spotifyPackageName); err == nil {
		// If the package is already visible,
		// needs to close it and launch again to collect app start time.
		if err := w.CloseWindow(ctx, tconn); err != nil {
			return -1, errors.Wrapf(err, "failed to close %s app", SpotifyAppName)
		}
	}

	var startTime time.Time
	// Sometimes the Spotify App will fail to open, so add retry here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.SearchAndLaunch(tconn, s.kb, SpotifyAppName)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s app", SpotifyAppName)
		}
		startTime = time.Now()
		return ash.WaitForVisible(ctx, tconn, spotifyPackageName)
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return -1, errors.Wrapf(err, "failed to wait for the new window of %s", spotifyPackageName)
	}

	return time.Since(startTime), nil
}

// Close closes Soptify app.
func (s *Spotify) Close(ctx context.Context, tconn *chrome.TestConn) error {
	w, err := ash.GetARCAppWindowInfo(ctx, tconn, spotifyPackageName)
	if err != nil {
		return errors.Wrap(err, "failed to get Spotify window info")
	}
	return w.CloseWindow(ctx, tconn)
}

// Play plays a song.
func (s *Spotify) Play(ctx context.Context) error {
	if err := s.login(ctx); err != nil {
		return errors.Wrap(err, "failed to login into Spotify")
	}

	testing.ContextLog(ctx, "Clearing prompts")
	dismiss := s.d.Object(ui.Text("DISMISS"))
	promp := s.d.Object(ui.Text("NO, THANKS"))
	if err := uiauto.Combine("clear prompt",
		cuj.ClickIfExistAction(dismiss, s.uiTimeout),
		cuj.ClickIfExistAction(promp, s.uiTimeout),
	)(ctx); err != nil {
		return err
	}

	if err := s.waitUntilHomePageShows(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until home page shows")
	}

	testing.ContextLog(ctx, "Try to play a song")
	const playPauseButtonID = spotifyIDPrefix + "play_pause_button"
	playButton := s.d.Object(ui.ID(playPauseButtonID), ui.Enabled(true), ui.Description("Play"))

	if err := playButton.WaitForExists(ctx, s.uiTimeout); err != nil {
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
	pauseButton := s.d.Object(ui.ID(playPauseButtonID), ui.Enabled(true), ui.Description("Pause"))
	if err := pauseButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait for pause button to exist, Spotify is not playing")
	}

	testing.ContextLog(ctx, "Spotify is playing")
	return nil
}

func (s *Spotify) login(ctx context.Context) error {
	testing.ContextLog(ctx, "Signing into Spotify")
	signIn := s.d.Object(ui.Text("Continue with Google"))
	if err := signIn.WaitForExists(ctx, s.uiTimeout); err != nil {
		testing.ContextLog(ctx, `"Continue with Google" button not found, assuming splash screen has been dismissed already`)
	} else if err := signIn.Click(ctx); err != nil {
		return errors.Wrap(err, `failed to click "Continue with Google" button`)
	} else {
		accountButton := s.d.Object(ui.Text(s.account))
		if err := cuj.FindAndClick(ctx, accountButton, s.uiTimeout); err != nil {
			testing.ContextLog(ctx, `The button "account button" not found, sign in directly`)
		}
		s.firstLogin = true
	}

	return nil
}

func (s *Spotify) waitUntilHomePageShows(ctx context.Context) error {
	searchTabID := spotifyIDPrefix + "search_tab"
	searchTab := s.d.Object(ui.ID(searchTabID))

	if err := searchTab.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, `failed to wait for search tab exist`)
	}

	return nil
}

func (s *Spotify) playLastListenedSong(ctx context.Context, playButton *ui.Object) error {
	testing.ContextLog(ctx, "Try to play last listened song")

	if err := cuj.FindAndClickAction(playButton, s.uiTimeout)(ctx); err != nil {
		testing.ContextLog(ctx, `Failed to play last listened song, try to search a song and play`)
		return s.searchSongAndPlay(ctx)
	}

	return nil
}

func (s *Spotify) searchSongAndPlay(ctx context.Context) error {
	const (
		searchTabID   = spotifyIDPrefix + "search_tab"
		searchFieldID = spotifyIDPrefix + "find_search_field_text"
		queryID       = spotifyIDPrefix + "query"
		childrenID    = spotifyIDPrefix + "children"
		albumName     = "Photograph"
		singerName    = "Song • Ed Sheeran"
	)

	var (
		searchTab    = s.d.Object(ui.ID(searchTabID))
		searchField  = s.d.Object(ui.ID(searchFieldID))
		query        = s.d.Object(ui.ID(queryID))
		singerButton = s.d.Object(ui.Text(singerName))
	)

	testing.ContextLog(ctx, "Try to search a song and play")

	if err := uiauto.Combine("search song",
		cuj.FindAndClickAction(searchTab, defaultUITimeout),
		cuj.FindAndClickAction(searchField, defaultUITimeout),
		cuj.FindAndClickAction(query, defaultUITimeout),
		s.kb.TypeAction(albumName),
		cuj.FindAndClickAction(singerButton, defaultUITimeout),
	)(ctx); err != nil {
		return err
	}

	var shufflePlayButton *ui.Object
	if s.firstLogin {
		shufflePlayButton = s.d.Object(ui.Text("SHUFFLE PLAY"))
	} else {
		shufflePlayButton = s.d.Object(ui.ID(childrenID), ui.ClassName("android.widget.LinearLayout"))
	}

	// It might automatically start playing after click singerButton,
	// so skip if shufflePlayButton not found.
	if err := cuj.ClickIfExist(ctx, shufflePlayButton, defaultUITimeout); err != nil {
		return errors.Wrap(err, `failed to click "shuffle play button"`)
	}

	return nil
}
