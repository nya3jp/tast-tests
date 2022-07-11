// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package youtubeweb provides a library for controlling youtube web.
package youtubeweb

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
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

const (
	mouseMoveDuration = 500 * time.Millisecond
)

// youtubeWeb implements the interface mediaPlayer.
type youtubeWeb struct {
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	browserRoot *nodewith.Finder
	conn        *chrome.Conn

	cr     *chrome.Chrome
	outDir string
}

// New returns an instance of youtubeWeb struct.
// cr and outDir are for dumping faillog.
func New(tconn *chrome.TestConn, browserType browser.Type, cr *chrome.Chrome, outDir string) *youtubeWeb {
	yw := &youtubeWeb{
		tconn:  tconn,
		ui:     uiauto.New(tconn),
		cr:     cr,
		outDir: outDir,
	}

	if browserType == browser.TypeLacros {
		classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
		yw.browserRoot = nodewith.Role(role.Window).ClassNameRegex(classNameRegexp).NameContaining("YouTube")
	} else {
		yw.browserRoot = nodewith.Role(role.Window).HasClass("BrowserFrame").NameContaining("YouTube")
	}

	return yw
}

// Open opens the youtube web page.
func (yw *youtubeWeb) Open(ctx context.Context, br *browser.Browser, url string) (retErr error) {
	if yw.conn != nil {
		return nil
	}

	var err error
	if yw.conn, err = br.NewConn(ctx, url); err != nil {
		return errors.Wrap(err, "failed to open youtube")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		if retErr != nil {
			yw.Close(ctx, func() bool { return true })
		}
	}(cleanupCtx)

	return webutil.WaitForYoutubeVideo(ctx, yw.conn, 30*time.Second)
}

// Close closes the youtube web page.
func (yw *youtubeWeb) Close(ctx context.Context, hasError func() bool) error {
	if yw.conn == nil {
		return nil
	}

	faillog.DumpUITreeWithScreenshotOnError(ctx, yw.outDir, hasError, yw.cr, "yt_web")

	hasCloseErr := false
	if err := yw.conn.CloseTarget(ctx); err != nil {
		hasCloseErr = true
		testing.ContextLog(ctx, "Failed to close youtube web page: ", err)
	}
	if err := yw.conn.Close(); err != nil {
		hasCloseErr = true
		testing.ContextLog(ctx, "Failed to close youtube connection: ", err)
	}

	if hasCloseErr {
		return errors.New("failed to close youtube")
	}

	yw.conn = nil
	return nil
}

// Play plays the youtube.
func (yw *youtubeWeb) Play(ctx context.Context) error {
	if status, err := yw.MediaStatus(ctx); err != nil {
		return errors.Wrap(err, "failed to check if it's playing")
	} else if status == Playing {
		return nil
	}

	if err := yw.skipAd(ctx); err != nil {
		return errors.Wrap(err, "failed to skip ad")
	}

	// Play button and pause button share the identical name.
	playPauseBtn := nodewith.Name("Pause (k)").Role(role.Button).Ancestor(videoPlayerContainer)

	return uiauto.Combine("play video",
		yw.MouseMoveTo(videoPlayerContainer, mouseMoveDuration),
		yw.LeftClick(playPauseBtn),
	)(ctx)
}

// WaitUntilPlaying waits until youtube is playing.
func (yw *youtubeWeb) WaitUntilPlaying(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := yw.MediaStatus(ctx); err != nil {
			return testing.PollBreak(err)
		} else if status != Playing {
			return errors.New("youtube is not playing")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// WaitUntilPaused waits until youtube is paused.
func (yw *youtubeWeb) WaitUntilPaused(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := yw.MediaStatus(ctx); err != nil {
			return testing.PollBreak(err)
		} else if status != Paused {
			return errors.New("youtube is not paused")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// RetrieveCurrentTime retrieves the current time of the youtube.
func (yw *youtubeWeb) RetrieveCurrentTime(ctx context.Context) (time.Duration, error) {
	videoLocation, err := yw.Location(ctx, videoPlayerContainer)
	if err != nil {
		return 0, err
	}

	timeNode := nodewith.NameRegex(regexp.MustCompile("^(\\d+):(\\d+)(:\\d+)?$")).Role(role.InlineTextBox).First().Ancestor(videoPlayerContainer)
	if err := yw.skipAd(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to skip youtube add")
	}

	flag := false
	mouseMoveAction := func(ctx context.Context) error {
		flag = !flag
		if flag {
			return mouse.Move(yw.tconn, videoLocation.TopLeft(), mouseMoveDuration)(ctx)
		}
		return mouse.Move(yw.tconn, videoLocation.CenterPoint(), mouseMoveDuration)(ctx)
	}

	// Retry moving mouse on player container to show the control panel.
	if err := yw.RetryUntil(
		mouseMoveAction,
		yw.controlPanelShows,
	)(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to ensure control panel show up")
	}

	// Once the control panel has been shown, the timeNode will be updated.
	node, err := yw.Info(ctx, timeNode)
	if err != nil {
		return 0, err
	}
	testing.ContextLogf(ctx, "Current time: %s", node.Name)

	tc := node.Name + "s"
	if strings.Count(tc, ":") == 1 {
		tc = strings.Replace(tc, ":", "m", 1)
	} else if strings.Count(tc, ":") == 2 {
		tc = strings.Replace(tc, ":", "h", 1)
		tc = strings.Replace(tc, ":", "m", 1)
	} else {
		return 0, errors.Errorf(`unexpected time code, want: "hh:mm:ss" or "mm:ss", got: %q`, node.Name)
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
func (yw *youtubeWeb) MediaStatus(ctx context.Context) (PlayStatus, error) {
	info, err := yw.Info(ctx, videoPlayerContainer)
	if err != nil {
		return Unknown, errors.Wrap(err, "failed to obtain the node's information")
	}
	status := info.ClassName

	if strings.Contains(status, "unstarted-mode") || strings.Contains(status, "paused-mode") {
		return Paused, nil
	} else if strings.Contains(status, "playing-mode") {
		return Playing, nil
	}

	return Unknown, errors.Errorf("unknow media status: %q", status)
}

// skipAd clicks the "Skip Ad" button to skip the ad.
func (yw *youtubeWeb) skipAd(ctx context.Context) error {
	var (
		adNodesFinder   = nodewith.Ancestor(videoPlayerContainer)
		skipAd          = adNodesFinder.NameContaining("Skip Ad").Role(role.Button)
		adLinkButton    = adNodesFinder.HasClass("ytp-ad-button ytp-ad-visit-advertiser-button ytp-ad-button-link").Role(role.Button)
		numberOfAdsText = adNodesFinder.NameRegex(regexp.MustCompile(`Ad \d of \d`)).Role(role.StaticText)
	)

	return testing.Poll(ctx, func(ctx context.Context) error {
		foundAdLinkButton, err := yw.IsNodeFound(ctx, adLinkButton)
		if err != nil {
			return errors.Wrap(err, "failed to find ad link button")
		}
		foundNumberOfAdsText, err := yw.IsNodeFound(ctx, numberOfAdsText)
		if err != nil {
			return errors.Wrap(err, "failed to find number of ad information")
		}
		foundSkipAdButton, err := yw.IsNodeFound(ctx, skipAd)
		if err != nil {
			return errors.Wrap(err, "failed to find `Skip Ads` button")
		}
		if foundAdLinkButton || foundNumberOfAdsText || foundSkipAdButton {
			testing.ContextLog(ctx, "Ad appears")
			if err := uiauto.IfSuccessThen(
				yw.WithTimeout(5*time.Second).WaitUntilExists(skipAd),
				uiauto.NamedAction(`click the "Skip Ad" button`, yw.LeftClick(skipAd)),
			)(ctx); err != nil {
				return testing.PollBreak(err)
			}
			return errors.New("Ad is playing")
		}

		testing.ContextLog(ctx, "Ad is skipped or not existed")
		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}

// controlPanelShows checks and returns nil if the control panel appears.
// The control panel is visible if the class list of player element doesn't includes class "ytp-autohide".
// Applied Javascript due to an UI tree sychronize latency issue of the control panel state.
// The class containing control panel transition state from hide to show may exist in UI tree for longer than expected.
func (yw *youtubeWeb) controlPanelShows(ctx context.Context) error {
	return yw.conn.WaitForExprFailOnErrWithTimeout(ctx, `() => {
		let e = document.querySelector("#movie_player");
		return !e.classList.contains("ytp-autohide");
	}`, 5*time.Second)
}
