// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package videocuj

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func openAndPlayYoutubeWeb(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, video Video) (*chrome.Conn, error) {
	const timeout = time.Second * 15
	testing.ContextLog(ctx, "Open Youtube web")
	ytConn, err := cr.NewConn(ctx, video.url, cdputil.WithNewWindow())
	if err != nil {
		return nil, errors.Wrap(err, "failed to open youtube")
	}

	if err := webutil.WaitForYoutubeVideo(ctx, ytConn, 0); err != nil {
		return nil, errors.Wrap(err, "failed to wait for video element")
	}

	switchQuality := func(resolution string) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			node, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "YouTube Video Player"}, 15*time.Second)
			if err != nil {
				return errors.Wrap(err, "failed to find 'YouTube Video Player'")
			}
			defer node.Release(ctx)

			c := coords.Point{
				X: node.Location.Left + node.Location.Width/2,
				Y: node.Location.Top + node.Location.Height - 1,
			}

			// Mouse move to show 'Settings'
			if err := mouse.Move(ctx, tconn, c, 0); err != nil {
				return errors.Wrap(err, "failed to move the mouse")
			}

			testing.ContextLog(ctx, `Click 'Settings'`)
			settingsParams := ui.FindParams{Role: ui.RoleTypePopUpButton, Name: "Settings"}
			if err := cuj.WaitAndClick(ctx, tconn, settingsParams, timeout); err != nil {
				return errors.Wrap(err, "failed to click 'Settings'")
			}
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: time.Second * 20}); err != nil {
			return errors.Wrap(err, "failed to switch Quality")
		}

		testing.ContextLog(ctx, `Click 'Quality'`)
		if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{
			Attributes: map[string]interface{}{"name": regexp.MustCompile(`^Quality`)},
			Role:       ui.RoleTypeMenuItem}, timeout); err != nil {
			return errors.Wrap(err, "failed to click 'Quality'")
		}
		testing.ContextLogf(ctx, "Click %q", resolution)
		resolutionParams := ui.FindParams{
			Attributes: map[string]interface{}{
				"name": regexp.MustCompile(fmt.Sprintf("^%s", resolution)),
			},
			Role: ui.RoleTypeMenuItemRadio,
		}
		if err := cuj.WaitAndClick(ctx, tconn, resolutionParams, timeout); err != nil {
			return errors.Wrapf(err, "failed to click %q", resolution)
		}
		testing.ContextLog(ctx, "Verify youtube is ready to play")
		if err := waitForYoutubeReadyState(ctx, ytConn); err != nil {
			return errors.Wrap(err, "failed to wait for Youtube ready state")
		}

		return nil
	}

	testing.ContextLog(ctx, "Switch to the new quality")
	if err := switchQuality(video.quality); err != nil {
		return nil, errors.Wrapf(err, "failed to switch resolution to %s", video.quality)
	}
	return ytConn, nil
}

func enterYoutubeWebFullscreen(ctx context.Context, tconn *chrome.TestConn, ytConn *chrome.Conn, ytWinID int) error {
	testing.ContextLog(ctx, "Make Youtube video fullscreen")
	getYtElemBounds := func(sel string) (coords.Rect, error) {
		var bounds coords.Rect
		if err := ytConn.Eval(ctx, fmt.Sprintf(
			`(function() {
					  var b = document.querySelector(%q).getBoundingClientRect();
						return {
							'left': Math.round(b.left),
							'top': Math.round(b.top),
							'width': Math.round(b.width),
							'height': Math.round(b.height),
						};
					})()`,
			sel), &bounds); err != nil {
			return coords.Rect{}, errors.Wrapf(err, "failed to get bounds for selector %q", sel)
		}

		return bounds, nil
	}
	// Returns bounds of the element when the element does not change its bounds
	// and is at the top (a tap/click should reach it).
	getTappableYtElemBounds := func(sel string) (coords.Rect, error) {
		var bounds coords.Rect

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if newBounds, err := getYtElemBounds(sel); err != nil {
				return err
			} else if newBounds != bounds {
				bounds = newBounds
				return errors.New("bounds are changing")
			}

			if bounds.Width == 0 || bounds.Height == 0 {
				return errors.Errorf("bad bound for selector %q", sel)
			}

			var atTop bool
			if err := ytConn.Eval(ctx, fmt.Sprintf(
				`(function() {
							var sel = document.querySelector(%q);
							var el = document.elementFromPoint(%d, %d);
							return sel.contains(el);
						})()`,
				sel, bounds.CenterPoint().X, bounds.CenterPoint().Y),
				&atTop); err != nil {
				return errors.Wrapf(err, "failed to check at top of selector %q", sel)
			}
			if !atTop {
				return errors.Errorf("selector %q is not at top", sel)
			}

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return coords.Rect{}, err
		}

		return bounds, nil
	}

	tapYtElem := func(sel string) error {
		bounds, err := getTappableYtElemBounds(sel)
		if err != nil {
			return err
		}

		all, err := ui.FindAll(ctx, tconn,
			ui.FindParams{
				ClassName: "WebContentsViewAura",
				Role:      ui.RoleTypeWindow})
		if err != nil {
			return errors.Wrap(err, "failed to find WebContentsViewAura node")
		}
		defer all.Release(ctx)

		var ytWeb *ui.Node
		for _, n := range all {
			if strings.Contains(n.Name, "YouTube") {
				ytWeb = n
				break
			}
		}
		if ytWeb == nil {
			return errors.Wrap(err, "failed to find YouTube WebContentsViewAura node")
		}

		tapPoint := bounds.CenterPoint().Add(ytWeb.Location.TopLeft())

		pc := pointer.NewMouseController(tconn)
		defer pc.Close()
		if err := pointer.Click(ctx, pc, tapPoint); err != nil {
			return errors.Wrapf(err, "failed to tap selector %q", sel)
		}
		return nil
	}

	tapFullscreenButton := func() error {
		if err := tapYtElem(`.ytp-fullscreen-button`); err != nil {
			// The failure could be caused by promotion banner covering the button.
			// It could happen in small screen devices. Attempt to dismiss the banner.
			// Ignore the error since the banner might not be there.
			if err := tapYtElem("ytd-button-renderer#dismiss-button"); err != nil {
				testing.ContextLog(ctx, "Failed to dismiss banner: ", err)
			}

			// Attempt to dismiss floating surveys that could  cover the bottom-right
			// and ignore the errors since the survey could not be there..
			if err := tapYtElem("button[aria-label='Dismiss']"); err != nil {
				testing.ContextLog(ctx, "Failed to dismiss survey: ", err)
			}

			// Tap the video to pause it to ensure the fullscreen button showing up.
			if err := tapYtElem(`video`); err != nil {
				return errors.Wrap(err, "failed to tap video to pause it")
			}

			// Tap fullscreen button again.
			if err := tapYtElem(`.ytp-fullscreen-button`); err != nil {
				return errors.Wrap(err, "failed to tap fullscreen button")
			}

			if err := tapYtElem(`video`); err != nil {
				return errors.Wrap(err, "failed to tap video to resume it")
			}
		}

		return nil
	}

	enterFullscreen := func() error {
		if ytWin, err := ash.GetWindow(ctx, tconn, ytWinID); err != nil {
			return errors.Wrap(err, "failed to get youtube window")
		} else if ytWin.State == ash.WindowStateFullscreen {
			return errors.New("alreay in fullscreen")
		}

		// Clear notification prompts if exists
		cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Block", Role: ui.RoleTypeButton}, time.Second)

		if err := tapFullscreenButton(); err != nil {
			return err
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == ytWinID && w.State == ash.WindowStateFullscreen
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to tapFullscreenButton(): ")
		}

		return nil
	}

	if err := enterFullscreen(); err != nil {
		return errors.Wrap(err, "failed to enterFullscreen(): ")
	}

	if err := waitForYoutubeReadyState(ctx, ytConn); err != nil {
		return errors.Wrap(err, "failed to wait for Youtube ready state")
	}
	return nil
}

func pauseAndPlayYoutubeWeb(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		playButton  = "Play (k)"
		pauseButton = "Pause (k)"
		timeout     = time.Second * 15
		waitTime    = time.Second * 3
	)
	pauseParams := ui.FindParams{
		Name: pauseButton,
		Role: ui.RoleTypeButton,
	}
	playParams := ui.FindParams{
		Name: playButton,
		Role: ui.RoleTypeButton,
	}

	testing.ContextLog(ctx, "Checking the playing status of youtube video")
	if err := ui.WaitUntilExists(ctx, tconn, pauseParams, timeout); err != nil {
		return errors.Wrap(err, "failed to find pause button to check video is playing")
	}

	testing.ContextLog(ctx, "Click pause button")
	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: pauseButton, Role: ui.RoleTypeButton}, timeout); err != nil {
		return errors.Wrap(err, "failed to click 'pause' button")
	}

	testing.ContextLog(ctx, "Verify youtube video is paused")
	if err := ui.WaitUntilExists(ctx, tconn, playParams, timeout); err != nil {
		return errors.Wrap(err, "failed to find play button to check video is playing")
	}

	// Wait time to see the video is paused
	testing.Sleep(ctx, waitTime)

	testing.ContextLog(ctx, "Click play button")
	if err := cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: playButton, Role: ui.RoleTypeButton}, timeout); err != nil {
		return errors.Wrap(err, "failed to click 'play' button")
	}
	testing.ContextLog(ctx, "Verify youtube video is playing")
	if err := ui.WaitUntilExists(ctx, tconn, pauseParams, timeout); err != nil {
		return errors.Wrap(err, "failed to find pause button to check video is playing")
	}

	// Wait time to see the video is playing
	testing.Sleep(ctx, waitTime)

	return nil
}

// waitForYoutubeReadyState does wait youtube video ready state then return.
func waitForYoutubeReadyState(ctx context.Context, conn *chrome.Conn) error {
	// VideoPlayer represents the main <video> node in youtube page.
	const VideoPlayer = "#movie_player > div.html5-video-container > video"
	queryCode := fmt.Sprintf("new Promise((resolve, reject) => { let video = document.querySelector(%q); resolve(video.readyState === 4 && video.buffered.length > 0); });", VideoPlayer)

	// Wait for element to appear.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var pageState bool
		if err := conn.EvalPromise(ctx, queryCode, &pageState); err != nil {
			return err
		}
		if pageState {
			return nil
		}
		return errors.New("failed to wait for ready state")
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
		return err
	}
	return nil
}
