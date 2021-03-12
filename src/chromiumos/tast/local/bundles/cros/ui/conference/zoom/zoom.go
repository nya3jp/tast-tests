// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package zoom

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type zoomConference struct {
	cr      *chrome.Chrome
	account string
}

func (conf *zoomConference) Join(ctx context.Context, tconn *chrome.TestConn, room string) error {
	const timeout = time.Second * 15

	conn, err := conf.cr.NewConn(ctx, room)
	if err != nil {
		return errors.Wrap(err, "failed to open the zoom website")
	}
	defer conn.Close()

	testing.ContextLog(ctx, "Click 'Join from Your Browser' link")
	if err := conference.ClickUIByName(ctx, tconn, "Join from Your Browser", timeout); err != nil {
		return err
	}

	if err := conference.WaitUIByName(ctx, tconn, "Join", timeout); err != nil {
		testing.ContextLog(ctx, "Failed to wait 'Join' button")
	}
	if err := conference.WaitUIByName(ctx, tconn, "SIGN IN", time.Second*3); err == nil {
		testing.ContextLog(ctx, "Start to sign in")
		if err := conn.Navigate(ctx, "https://zoom.us/google_oauth_signin"); err != nil {
			return err
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			conference.ClickUIByName(ctx, tconn, conf.account, time.Second)
			if err := conference.WaitUIByName(ctx, tconn, "Profile picture", time.Second*3); err != nil {
				return errors.Wrap(err, "failed to wait 'Profile picture'")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			return err
		}

		if err := conn.Navigate(ctx, room); err != nil {
			return err
		}

		testing.ContextLog(ctx, "Click 'Join from Your Browser' link")
		if err := conference.ClickUIByName(ctx, tconn, "Join from Your Browser", timeout); err != nil {
			return err
		}
	} else {
		testing.ContextLog(ctx, "It has been signed in")
	}

	testing.ContextLog(ctx, "Click 'Join' button")
	if err := conference.ClickUIByName(ctx, tconn, "Join", timeout); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Wait 'Join Audio by Computer' button")
	if err := conference.WaitUIByName(ctx, tconn, "Join Audio by Computer", time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait 'Join Audio by Computer' button")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		conference.ClickUIByName(ctx, tconn, "Join Audio by Computer", time.Second*3)
		if err := conference.WaitUIByName(ctx, tconn, "Join Audio by Computer", time.Second); err == nil {
			return errors.Wrap(err, "'Join Audio by Computer' button shouldn't be shown")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return err
	}

	// Allow audio permission
	conference.ClickUIByName(ctx, tconn, "Allow", time.Second*3)
	if err := toggleVideo(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}

	// Allow video permission
	conference.ClickUIByName(ctx, tconn, "Allow", time.Second*3)
	return nil
}

func (conf *zoomConference) VideoAudioControl(ctx context.Context, tconn *chrome.TestConn) error {
	const waitTime = time.Second * 5

	testing.ContextLog(ctx, "Toggle Video")
	if err := toggleVideo(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	testing.ContextLog(ctx, "Toggle Video")
	if err := toggleVideo(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	return nil
}

func (conf *zoomConference) SwitchTabs(ctx context.Context, tconn *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Open wiki page")
	wikiURL := "https://www.wikipedia.org/"
	wikiConn, err := conf.cr.NewConn(ctx, wikiURL)
	if err != nil {
		return errors.Wrap(err, "failed to open the wiki url")
	}
	defer wikiConn.Close()

	// switch tab
	if err := kb.Accel(ctx, "ctrl+tab"); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}

	return nil
}

func (conf *zoomConference) ChangeLayout(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		view     = "View"
		speaker  = "Speaker View"
		gallery  = "Gallery View"
		waitTime = time.Second * 5
	)

	if err := conference.ClickUIByName(ctx, tconn, view, waitTime); err != nil {
		// There are some DUT didn't show View button
		testing.ContextLog(ctx, "This DUT didn't show View button, pass ChangeLayout")
		return nil
	}
	if err := conference.ClickUIByName(ctx, tconn, speaker, waitTime); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	if err := conference.ClickUIByName(ctx, tconn, view, waitTime); err != nil {
		return err
	}
	if err := conference.ClickUIByName(ctx, tconn, gallery, waitTime); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	return nil
}

func (conf *zoomConference) BackgroundBlurring(ctx context.Context, tconn *chrome.TestConn) error {
	// Zoom doesn't support background change in web
	return nil
}

func (conf *zoomConference) ExtendedDisplayPresenting(_ context.Context, _ *chrome.TestConn, _ bool) error {
	return nil
}

func (conf *zoomConference) PresentSlide(ctx context.Context, tconn *chrome.TestConn) error {
	const timeout = time.Second * 15

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		return errors.Wrap(err, "failed to find webview")
	}
	defer webview.Release(ctx)

	// Make Zoom to show the bottom bar
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Click present now button")
	if err := kb.Accel(ctx, "tab"); err != nil {
		return errors.Wrap(err, "failed to send keyboard event")
	}
	if err := kb.Accel(ctx, "tab"); err != nil {
		return errors.Wrap(err, "failed to send keyboard event")
	}

	if err := testing.Sleep(ctx, time.Second*5); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.WaitUIByName(ctx, tconn, "Share Screen", timeout); err != nil {
			testing.ContextLog(ctx, "Failed to wait 'Share Screen' button")
			return err
		}

		if err := conference.ClickUIByName(ctx, tconn, "Share Screen", timeout); err != nil {
			testing.ContextLog(ctx, "Click 'Share Screen' button")
			return err
		}

		testing.ContextLog(ctx, "Select window to present")
		if err := testing.Sleep(ctx, time.Second*2); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}

		if err := kb.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to type the tab key")
		}
		if err := kb.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to type the tab key")
		}
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to type the enter key")
		}

		if err := conference.WaitUIByName(ctx, tconn, "Stop Share", timeout); err != nil {
			testing.ContextLog(ctx, "Failed to wait 'Stop presenting' button")
			return err
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrap(err, `failed to click Share Screen button`)
	}

	testing.ContextLog(ctx, "Open a slide")
	slideURL := "https://docs.google.com/presentation/d/1BuvbMyZ0KE_kgtJ3WODZe0dXz2hs2qrjgM82NxhIQos/edit"
	slideConn, err := conf.cr.NewConn(ctx, slideURL)
	if err != nil {
		return errors.Wrap(err, "failed to open the slide url")
	}
	defer slideConn.Close()

	testing.ContextLog(ctx, "Start present")
	conference.ClickUIByName(ctx, tconn, "OK", time.Second)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		present, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Present"}, time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to present slide")
		}
		defer present.Release(ctx)
		if err := present.LeftClick(ctx); err != nil {
			return errors.Wrap(err, `failed to click "Present" button`)
		}
		if err := webview.WaitUntilDescendantGone(ctx, ui.FindParams{Name: "Present"}, timeout); err != nil {
			return errors.New("present button does not disappear")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "failed to present slide")
	}

	testing.ContextLog(ctx, "Switch slides")
	for i := 0; i < 6; i++ {
		if err := kb.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, `failed to type enter key to switch slide`)
		}
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
	}

	testing.ContextLog(ctx, "Leave presentation mode")
	if err := kb.Accel(ctx, "Esc"); err != nil {
		return errors.Wrap(err, `failed to type esc to leave presentation mode`)
	}

	if err := testing.Sleep(ctx, time.Second*2); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	testing.ContextLog(ctx, "Edit slide")
	if err := conference.EditSlide(ctx, tconn, kb); err != nil {
		return errors.Wrap(err, `failed to edit slide when leave presentation mode`)
	}

	// Switch back to conference page
	if err := kb.Accel(ctx, "ctrl+1"); err != nil {
		return errors.Wrap(err, `failed to type`)
	}

	return nil
}

func (conf *zoomConference) StopPresenting(ctx context.Context, tconn *chrome.TestConn) error {
	const timeout = time.Second * 20

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, timeout)
	if err != nil {
		return errors.Wrap(err, "failed to find webview")
	}
	defer webview.Release(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Stop presenting")
	if err := conference.ClickUIByName(ctx, tconn, "Stop Share", timeout); err != nil {
		return err
	}

	return nil
}

func (conf *zoomConference) End(ctx context.Context, tconn *chrome.TestConn) error {
	return cuj.CloseAllWindows(ctx, tconn)
}

var _ conference.Conference = (*zoomConference)(nil)

// NewZoomConference creates Zoom conference room instance which implements Conference interface.
func NewZoomConference(cr *chrome.Chrome, account string) *zoomConference {
	return &zoomConference{cr: cr, account: account}
}

func toggleVideo(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		stopVideo  = "stop sending my video"
		startVideo = "start sending my video"
	)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, stopVideo, time.Second*3); err != nil {
			if err := conference.ClickUIByName(ctx, tconn, startVideo, time.Second*3); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Open Video")
			return nil
		}
		testing.ContextLog(ctx, "Close Video")
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 20, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}

func toggleAudio(ctx context.Context, tconn *chrome.TestConn) error {
	const (
		muteAudio   = "mute my microphone"
		unmuteAudio = "unmute my microphone"
	)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conference.ClickUIByName(ctx, tconn, muteAudio, time.Second*3); err != nil {
			if err := conference.ClickUIByName(ctx, tconn, unmuteAudio, time.Second*3); err != nil {
				return err
			}
			testing.ContextLog(ctx, "Open Audio")
			return nil
		}
		testing.ContextLog(ctx, "Mute Audio")
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 20, Interval: time.Second}); err != nil {
		return err
	}
	return nil
}
