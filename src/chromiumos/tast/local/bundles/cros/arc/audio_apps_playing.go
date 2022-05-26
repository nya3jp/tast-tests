// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/spotify"
	"chromiumos/tast/local/arc/apputil/youtubemusic"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf"
	"chromiumos/tast/testing"
)

type audioAppType string

const (
	appSpotify audioAppType = spotify.AppName
	appYtMusic audioAppType = youtubemusic.AppName
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioAppsPlaying,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test and verify top ARC++ audio apps are working",
		Contacts:     []string{"vivian.tsai@cienet.com", "alfredyu@cienet.com", "cienet-development@googlegroups.com"},
		// Purposely leave the empty Attr here. MTBF tests are not included in mainline or crosbolt for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      mtbf.LoginReuseFixture,
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "spotify",
				Val:  appSpotify,
			}, {
				Name: "youtubemusic",
				Val:  appYtMusic,
			},
		},
	})
}

// AudioAppsPlaying verifies top ARC++ audio apps are working.
func AudioAppsPlaying(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	recorder, err := mtbf.NewRecorder(ctx)
	if err != nil {
		s.Fatal("Failed to start record performance: ", err)
	}
	defer recorder.Record(cleanupCtx, s.OutDir())

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	a := s.FixtValue().(*mtbf.FixtValue).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard controller: ", err)
	}
	defer kb.Close()

	var (
		app  apputil.ARCAudioPlayer
		song *apputil.Audio
	)

	switch s.Param().(audioAppType) {
	case appYtMusic:
		app, err = youtubemusic.New(ctx, kb, tconn, a)
		if err != nil {
			s.Fatal("Failed to create YouTube Music app instance: ", err)
		}
		song = apputil.NewAudio("Blank Space", "Taylor Swift • 3:51")
	case appSpotify:
		app, err = spotify.New(ctx, kb, a, tconn, cr.Creds().User)
		if err != nil {
			s.Fatal("Failed to create Spotify app instance: ", err)
		}
		song = apputil.NewAudio("Photograph", "Song • Ed Sheeran")
	default:
		s.Fatal("Unrecognized app type: ", s.Param())
	}

	if err := app.Install(ctx); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	if _, err := app.Launch(ctx); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
	defer app.Close(cleanupCtx, cr, s.HasError, s.OutDir())

	if err := app.Play(ctx, song); err != nil {
		s.Fatal("Failed to play song: ", err)
	}

	if ytm, ok := s.Param().(*youtubemusic.YouTubeMusic); ok {
		if err := ytm.RemovePlayRecord(cleanupCtx); err != nil {
			s.Fatal("Failed to remove play record: ", err)
		}
	}
}
