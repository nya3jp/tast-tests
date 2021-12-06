// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

// Video contains all required resources of video object.
type Video struct {
	conn           *chrome.Conn
	url            string
	ui             *uiauto.Context
	playerFinder   *nodewith.Finder // playerFinder is used to locate player UI node via uiauto.
	playerSelector string           // playerSelector is used to control or acquire player element via JavaScript.
}

// New create a new Video instance. conn is not initialized yet at this stage.
func New(tconn *chrome.TestConn, url, playerSelector string, playerFinder *nodewith.Finder) *Video {
	return &Video{
		url:            url,
		ui:             uiauto.New(tconn),
		playerFinder:   playerFinder,
		playerSelector: playerSelector,
	}
}

// Open opens a video page with provided URL.
func (v *Video) Open(ctx context.Context, br *browser.Browser) (retErr error) {
	if v.conn != nil {
		return errors.New("video has been opened already")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var err error
	if v.conn, err = br.NewConn(ctx, v.url); err != nil {
		return errors.Wrapf(err, "failed to open video page with URL %q ", v.url)
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			v.conn.CloseTarget(ctx)
			v.conn.Close()
		}
	}(cleanupCtx)

	if err := v.WaitUntilVideoReady(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until video is ready")
	}

	return nil
}

// Close close video tab. This function should be called as defer function.
func (v *Video) Close(ctx context.Context) {
	if v.conn == nil {
		return
	}
	testing.ContextLog(ctx, "Closing video tab and object")
	if err := v.conn.CloseTarget(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close video tab: ", err)
	}
	if err := v.conn.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close video connection: ", err)
	}
	v.conn = nil
}

// WaitUntilVideoReady waits until video is ready.
func (v *Video) WaitUntilVideoReady(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		var ready bool
		if err := v.conn.Call(ctx, &ready, v.generateExpr(isVideoReady)); err != nil {
			return err
		}

		if !ready {
			return errors.New("video is not ready")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}

// Play plays and verifies video playing.
func (v *Video) Play(ctx context.Context) error {
	if err := v.conn.Call(ctx, nil, v.generateExpr(playVideo)); err != nil {
		return errors.Wrap(err, "failed to play video")
	}

	if isPlaying, err := v.IsPlaying(ctx); err != nil {
		return errors.Wrap(err, "failed to check if video is playing")
	} else if !isPlaying {
		return errors.New("video is not playing")
	}

	return nil
}

// Pause pauses and verifies video pausing.
func (v *Video) Pause(ctx context.Context) error {
	if err := v.conn.Call(ctx, nil, v.generateExpr(pauseVideo)); err != nil {
		return errors.Wrap(err, "failed to pause video")
	}

	if isPlaying, err := v.IsPlaying(ctx); err != nil {
		return errors.Wrap(err, "failed to check if video is playing")
	} else if isPlaying {
		return errors.New("video is not paused")
	}

	return nil
}

// Forward seeks video forward and verifies it.
func (v *Video) Forward(ctx context.Context) error {
	currentTime, err := v.CurrentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current time")
	}

	if err := v.conn.Call(ctx, nil, v.generateExpr(fastForward)); err != nil {
		return errors.Wrap(err, "failed to seek video forward")
	}

	if err := v.WaitUntilVideoReady(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until video is ready")
	}

	if isPlaying, err := v.IsPlaying(ctx); err != nil {
		return errors.Wrap(err, "failed to check if video is playing")
	} else if !isPlaying {
		return errors.New("video is not playing")
	}

	afterFastJumpTime, err := v.CurrentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current time after fast jump")
	}

	if (currentTime - afterFastJumpTime) >= fastJumpDuration {
		return errors.New("video is not fast forwarded")
	}

	return nil
}

// Rewind seeks video backward and verifies it.
func (v *Video) Rewind(ctx context.Context) error {
	currentTime, err := v.CurrentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current time")
	}

	if err := v.conn.Call(ctx, nil, v.generateExpr(fastRewind)); err != nil {
		return errors.Wrap(err, "failed to seek video backward")
	}

	if err := v.WaitUntilVideoReady(ctx); err != nil {
		return errors.Wrap(err, "failed to wait until video is ready")
	}

	if isPlaying, err := v.IsPlaying(ctx); err != nil {
		return errors.Wrap(err, "failed to check if video is playing")
	} else if !isPlaying {
		return errors.New("video is not playing")
	}

	afterFastJumpTime, err := v.CurrentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current time after fast jump")
	}

	if currentTime <= afterFastJumpTime {
		return errors.New("video is not fast rewound")
	}

	return nil
}

// CurrentTime returns current time of video.
func (v *Video) CurrentTime(ctx context.Context) (time float64, err error) {
	err = v.conn.Call(ctx, &time, v.generateExpr(getCurrentTime))
	return
}

// EnterFullScreen makes video enter full screen  and verifies it.
func (v *Video) EnterFullScreen(ctx context.Context) error {
	playing, err := v.IsPlaying(ctx)
	if err != nil {
		return err
	}

	// Trigger user action to avoid promise get rejected due to security reasons.
	if err := v.ui.LeftClick(v.playerFinder)(ctx); err != nil {
		return err
	}
	// The left click above might pause/play the video, restore the playing status when leaving this function.
	defer func() {
		if playing {
			v.Play(ctx)
		} else {
			v.Pause(ctx)
		}
	}()

	if err := v.conn.Call(ctx, nil, v.generateExpr(enterFullScreen)); err != nil {
		return err
	}

	if isFullScreen, err := v.IsFullScreen(ctx); err != nil {
		return errors.Wrap(err, "failed to check if video is full screen")
	} else if !isFullScreen {
		return errors.New("video is not full screen")
	}

	return nil
}

// ExitFullScreen makes video exit full screen and verifies it.
func (v *Video) ExitFullScreen(ctx context.Context) error {
	if err := v.conn.Call(ctx, nil, v.generateExpr(exitFullscreen)); err != nil {
		return err
	}

	if isFullScreen, err := v.IsFullScreen(ctx); err != nil {
		return errors.Wrap(err, "failed to check if video is full screen")
	} else if isFullScreen {
		return errors.New("video is not exited full screen")
	}

	return nil
}

// IsPlaying returns true if video is playing.
func (v *Video) IsPlaying(ctx context.Context) (bool, error) {
	var paused bool
	err := v.conn.Call(ctx, &paused, v.generateExpr(isVideoPaused))
	return !paused, err
}

// IsFullScreen returns true if video is in full screen mode.
func (v *Video) IsFullScreen(ctx context.Context) (bool, error) {
	var fullscreen bool
	err := v.conn.Call(ctx, &fullscreen, v.generateExpr(isFullscreen))
	return fullscreen, err
}

const (
	fastJumpDuration = 10
	interactWithDOM  = "element.muted = true"
)

type htmlAction string

const (
	playVideo       htmlAction = "play()"
	pauseVideo      htmlAction = "pause()"
	enterFullScreen htmlAction = "requestFullscreen()"
	exitFullscreen  htmlAction = "exitFullscreen()"
	isFullscreen    htmlAction = "webkitIsFullScreen"
	isVideoPaused   htmlAction = "paused"
	getCurrentTime  htmlAction = "currentTime"
	fastForward     htmlAction = "currentTime +="
	fastRewind      htmlAction = "currentTime -="
	isVideoReady    htmlAction = "element.readyState === 4 && element.buffered.length === 1"
)

// generateExpr generates a JavaScript expression according to different input htmlAction.
func (v *Video) generateExpr(act htmlAction) string {
	var (
		selectElement = fmt.Sprintf("element = %s", v.playerSelector)
		resolve       = fmt.Sprintf(`resolve(element.%s);`, act)
	)

	switch act {
	case exitFullscreen, isFullscreen:
		selectElement = `element = document`
	case fastForward, fastRewind:
		resolve = fmt.Sprintf(`element.%s %d; resolve();`, act, fastJumpDuration)
	case isVideoReady:
		resolve = fmt.Sprintf(`resolve(%s);`, act)
	}

	return fmt.Sprintf(`() => new Promise((resolve, reject) => {
		%[1]s;
		if (element !== null) {
			%[2]s;
			%[3]s;
		}
		reject(new Error('element not found'));
	})`, selectElement, interactWithDOM, resolve)
}
