// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package youtubemusic provides a library for controlling youtube music.
package youtubemusic

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

// youtubeMusic implements the interface mediaPlayer.
type youtubeMusic struct {
	ui          *uiauto.Context
	browserRoot *nodewith.Finder
	conn        *chrome.Conn

	cr     *chrome.Chrome
	outDir string
}

// New returns an instance of youtubeMusic struct.
// cr and outDir are for dumping faiilog.
func New(tconn *chrome.TestConn, browserType browser.Type, cr *chrome.Chrome, outDir string) *youtubeMusic {
	ytm := &youtubeMusic{
		ui:     uiauto.New(tconn),
		cr:     cr,
		outDir: outDir,
	}

	if browserType == browser.TypeLacros {
		classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
		ytm.browserRoot = nodewith.Role(role.Window).ClassNameRegex(classNameRegexp).NameContaining("YouTube Music")
	} else {
		ytm.browserRoot = nodewith.Role(role.Window).HasClass("BrowserFrame").NameContaining("YouTube Music")
	}

	return ytm
}

// Open opens the youtube music web page.
func (ytm *youtubeMusic) Open(ctx context.Context, br *browser.Browser, url string) (retErr error) {
	if ytm.conn != nil {
		return nil
	}

	var err error
	ytm.conn, err = br.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "failed to open youtube music")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		if retErr != nil {
			ytm.Close(ctx, func() bool { return true })
		}
	}(cleanupCtx)

	return webutil.WaitForYoutubeVideo(ctx, ytm.conn, 30*time.Second)
}

// Close closes the youtube music web page.
func (ytm *youtubeMusic) Close(ctx context.Context, hasError func() bool) error {
	if ytm.conn == nil {
		return nil
	}

	faillog.DumpUITreeWithScreenshotOnError(ctx, ytm.outDir, hasError, ytm.cr, "yt_music")

	hasCloseErr := false
	if err := ytm.conn.CloseTarget(ctx); err != nil {
		hasCloseErr = true
		testing.ContextLog(ctx, "Failed to close youtube music web page: ", err)
	}
	if err := ytm.conn.Close(); err != nil {
		hasCloseErr = true
		testing.ContextLog(ctx, "Failed to close youtube music connection: ", err)
	}

	if hasCloseErr {
		return errors.New("failed to close youtube music")
	}

	ytm.conn = nil
	return nil
}

// Play plays the youtube music.
func (ytm *youtubeMusic) Play(ctx context.Context) error {
	if status, err := ytm.MediaStatus(ctx); err != nil {
		return errors.Wrap(err, "failed to check if it's playing")
	} else if status == Playing {
		return nil
	}

	if err := ytm.LeftClick(playPauseButton.Name("Play"))(ctx); err != nil {
		return errors.Wrap(err, "failed to click play button")
	}

	adNodesFinder := nodewith.Ancestor(videoPlayerContainer)
	skipAdBtn := adNodesFinder.NameStartingWith("Skip Ad").Role(role.Button)

	adTextContainer := nodewith.HasClass("middle-controls style-scope ytmusic-player-bar").Role(role.GenericContainer).Ancestor(ytm.browserRoot)
	adText := nodewith.Name("Ad").Role(role.StaticText).Ancestor(adTextContainer)

	// There might be multiple ads, or ad without "Skip Ad" button.
	// Use polling to skip multiple ads, or wait the ad to finish.
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := uiauto.IfSuccessThen(
			ytm.WithTimeout(5*time.Second).WaitUntilExists(skipAdBtn),
			ytm.LeftClick(skipAdBtn),
		)(ctx); err != nil {
			return testing.PollBreak(err)
		}
		// Make sure the "Ad" text is gone.
		// Some ads are without skip button. Therefore, couldn't make the existence of skipAdBtn be a condition.
		return ytm.WithTimeout(5 * time.Second).WaitUntilGone(adText)(ctx)
	}, &testing.PollOptions{Interval: time.Second, Timeout: 30 * time.Second})
}

// WaitUntilPlaying waits until youtube music is playing.
func (ytm *youtubeMusic) WaitUntilPlaying(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := ytm.MediaStatus(ctx); err != nil {
			return testing.PollBreak(err)
		} else if status != Playing {
			return errors.New("youtube music is not playing")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// WaitUntilPaused waits until youtube music is paused.
func (ytm *youtubeMusic) WaitUntilPaused(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := ytm.MediaStatus(ctx); err != nil {
			return testing.PollBreak(err)
		} else if status != Paused {
			return errors.New("youtube music is not paused")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// RetrieveCurrentTime retrieves the current time of the youtube music.
func (ytm *youtubeMusic) RetrieveCurrentTime(ctx context.Context) (time.Duration, error) {
	timecodeFinder := nodewith.NameRegex(regexp.MustCompile("^(\\d+):(\\d+)(:\\d+)? / (\\d+):(\\d+)(:\\d+)?$")).Role(role.StaticText).Ancestor(playerBar)

	timecodeInfo, err := ytm.Info(ctx, timecodeFinder)
	if err != nil {
		return 0, errors.Wrap(err, "failed to obtain player time information")
	}

	timecode := regexp.MustCompile(" [/]").Split(timecodeInfo.Name, 2)[0]
	testing.ContextLog(ctx, "Player time: ", timecode)

	tc := timecode + "s"
	if strings.Count(tc, ":") == 1 {
		tc = strings.Replace(tc, ":", "m", 1)
	} else if strings.Count(tc, ":") == 2 {
		tc = strings.Replace(tc, ":", "h", 1)
		tc = strings.Replace(tc, ":", "m", 1)
	} else {
		return 0, errors.Errorf(`unexpected time code, want: "hh:mm:ss" or "mm:ss", got: %q`, timecode)
	}

	t, err := time.ParseDuration(tc)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse %s", tc)
	}

	return t, nil
}

// PlayStatus defines the media status.
type PlayStatus int

const (
	// Playing is the status of the playing media.
	Playing PlayStatus = iota
	// Paused is the status of the paused media.
	Paused
	// Unknown is the unknown media status.
	Unknown
)

// MediaStatus returns the instant media status.
func (ytm *youtubeMusic) MediaStatus(ctx context.Context) (PlayStatus, error) {
	info, err := ytm.Info(ctx, playPauseButton)
	if err != nil {
		return Unknown, errors.Wrap(err, "failed to obtain the node's information")
	}
	buttonName := info.Name

	switch buttonName {
	case "Play":
		return Paused, nil
	case "Pause":
		return Playing, nil
	default:
		return Unknown, errors.Errorf("unknow media status: %q", buttonName)
	}
}
