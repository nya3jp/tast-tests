// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/webmedia/vimeo"
	"chromiumos/tast/local/media/webmedia/youtube"
	"chromiumos/tast/local/mtbf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackSimultaneous,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Plays multiple videos simultaneously in different tabs",
		Contacts: []string{
			"abergman@google.com",
			"alfred.yu@cienet.com",
			"cj.tsai@cienet.com",
			"cienet-development@googlegroups.com",
		},
		SoftwareDeps: []string{"chrome"},
		// Purposely leave the empty Attr here. MTBF tests are not included in mainline or crosbolt for now.
		Attr:    []string{},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{
			{
				Fixture: mtbf.LoginReuseFixture,
				Val:     browser.TypeAsh,
			}, {
				Name:              "lacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Fixture:           mtbf.LoginReuseLacrosFixture,
				Val:               browser.TypeLacros,
			},
		},
	})
}

var (
	youtubeURL string = "https://www.youtube.com/watch?v=kJQP7kiw5Fk"
	vimeoURL   string = "https://vimeo.com/43401199"
)

// PlaybackSimultaneous verifies that multiple videos in different tabs can be played simultaneously.
func PlaybackSimultaneous(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	recorder, err := mtbf.NewRecorder(ctx)
	if err != nil {
		s.Fatal("Failed to start record performance: ", err)
	}
	defer recorder.Record(cleanupCtx, s.OutDir())

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	type videoPlayer interface {
		Open(context.Context, *browser.Browser) error
		Close(context.Context)
		Play(ctx context.Context) error
		IsPlaying(context.Context) (bool, error)
		CurrentTime(ctx context.Context) (time float64, err error)
		GetURL() string
	}

	// videoSources is the map of tab-order to video-source.
	videoSources := map[int]videoPlayer{
		1: youtube.New(tconn, youtubeURL),
		2: vimeo.New(tconn, vimeoURL),
	}

	// Open video sources by certain order.
	for order := 1; order <= len(videoSources); order++ {
		video := videoSources[order]

		if err := video.Open(ctx, br); err != nil {
			s.Fatalf("Failed to open video source [%s]: %v", video.GetURL(), err)
		}
		defer func(ctx context.Context) {
			faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("before_close_video_%d", order))
			video.Close(ctx)
		}(cleanupCtx)

		if err := video.Play(ctx); err != nil {
			s.Fatal("Failed to play video: ", err)
		}

		if playing, err := video.IsPlaying(ctx); err != nil {
			s.Fatal("Failed to check video is playing: ", err)
		} else if !playing {
			s.Fatal("Video isn't playing")
		}
	}

	// Close extra lacros tabs after all videos are opened.
	targets, err := br.FindTargets(ctx, chrome.MatchTargetURL(chrome.NewTabURL))
	if err != nil {
		s.Fatal("Failed to check the existence of empty tab: ", err)
	}
	for _, target := range targets {
		if err := br.CloseTarget(ctx, target.TargetID); err != nil {
			s.Fatal("Failed to close empty tab: ", err)
		}
	}

	// Switching between all tabs and verify all video sources are playing.
	for order := range videoSources {
		if err := kb.Accel(ctx, fmt.Sprintf("Ctrl+%d", order)); err != nil {
			s.Fatal("Failed to switch tab by shortcut: ", err)
		}
		s.Log("Switched to tab-", order)

		// Verify all video sources are playing.
		for order, video := range videoSources {
			if yt, ok := video.(*youtube.YouTube); ok {
				if err := yt.SkipAd(ctx); err != nil {
					s.Fatal("Failed to skip ads on YouTube: ", err)
				}
			}

			tBefore, err := video.CurrentTime(ctx)
			if err != nil {
				s.Fatalf("Failed to get video-%d current time: %v", order, err)
			}
			tAfter := tBefore

			// Checks the video is playing by monitoring the video time.
			pollOpt := testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if tAfter > tBefore {
					return nil
				}

				if tAfter, err = video.CurrentTime(ctx); err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get video time"))
				}
				return errors.New("video isn't playing")
			}, &pollOpt); err != nil {
				s.Fatalf("Video-%d isn't playing within %v: %v", order, pollOpt.Timeout, err)
			}
			s.Logf("Video-%d is playing", order)
		}
	}
}
