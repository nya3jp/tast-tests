// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ZoomConference implements the Conference interface.
type ZoomConference struct {
	cr         *chrome.Chrome
	tconn      *chrome.TestConn
	tabletMode bool
	account    string
}

// Join joins a new conference room.
func (conf *ZoomConference) Join(ctx context.Context, room string) error {
	ui := uiauto.New(conf.tconn)
	conn, err := conf.cr.NewConn(ctx, room)
	if err != nil {
		return errors.Wrap(err, "failed to open the zoom website")
	}
	defer conn.Close()

	testing.ContextLog(ctx, "Click 'Join from Your Browser' link")
	joinFromYourBrowser := nodewith.Name("Join from Your Browser").Role(role.Anchor)
	joinButton := nodewith.Name("Join").Role(role.Button)
	if err := uiauto.Combine("Join from Your Browser",
		ui.LeftClick(joinFromYourBrowser),
		ui.WaitUntilExists(joinButton),
	)(ctx); err != nil {
		return errors.Wrap(err, `failed to click 'Join from Your Browser'`)
	}

	if err = ui.WaitUntilExists(nodewith.Name("SIGN IN").Role(role.Link))(ctx); err == nil {
		testing.ContextLog(ctx, "Start to sign in")
		if err := conn.Navigate(ctx, "https://zoom.us/google_oauth_signin"); err != nil {
			return err
		}
		account := nodewith.Name(conf.account).First()
		profilePicture := nodewith.Name("Profile picture").First()
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// If the DUT has only one account, it would login to profile page directly.
			// Otherwise, it would show list of accounts.
			if err := ui.LeftClick(account)(ctx); err == nil {
				testing.ContextLog(ctx, "Select account in the account list")
			}
			if err := ui.WaitUntilExists(profilePicture)(ctx); err != nil {
				return errors.Wrap(err, "failed to wait 'Profile picture'")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Minute, Interval: 100 * time.Millisecond}); err != nil {
			return err
		}

		if err := conn.Navigate(ctx, room); err != nil {
			return err
		}

		testing.ContextLog(ctx, "Click 'Join from Your Browser' link")
		if err := ui.LeftClick(joinFromYourBrowser)(ctx); err != nil {
			return errors.Wrap(err, `failed to click 'Join from Your Browser'`)
		}
	} else {
		testing.ContextLog(ctx, "It has been signed in")
	}

	// Allow audio and video permission
	allowButton := nodewith.Name("Allow").Role(role.Button)
	if err := ui.WithTimeout(5 * time.Second).LeftClick(allowButton)(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to click allow button")
	}

	testing.ContextLog(ctx, "Join conference")
	joinAudioButton := nodewith.Name("Join Audio by Computer").Role(role.Button)
	if err := uiauto.Combine("Join conference",
		ui.LeftClick(joinButton),
		ui.WithTimeout(time.Second*30).WaitUntilExists(joinAudioButton),
		ui.LeftClickUntil(joinAudioButton, ui.Gone(joinAudioButton)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to join conference")
	}

	testing.ContextLog(ctx, "Start video")
	startVideoButton := nodewith.Name("start sending my video").Role(role.Button)
	stopVideoButton := nodewith.Name("stop sending my video").Role(role.Button)
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)

	// If kept idle, the zoom UI buttons may become invisible.
	// Click web area in order to make the video button reappear.
	if err := uiauto.Combine("Start video",
		ui.LeftClick(webArea),
		ui.LeftClick(startVideoButton),
		ui.WaitUntilExists(stopVideoButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to start video")
	}
	return nil
}

// VideoAudioControl controls the video and audio during conference.
func (conf *ZoomConference) VideoAudioControl(ctx context.Context) error {
	const waitTime = 5 * time.Second

	toggleVideo := func(ctx context.Context) error {
		const (
			stopVideo  = "stop sending my video"
			startVideo = "start sending my video"
		)
		ui := uiauto.New(conf.tconn)
		stopVideoButton := nodewith.Name(stopVideo).Role(role.Button)
		startVideoButton := nodewith.Name(startVideo).Role(role.Button)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			err := ui.ImmediateLeftClick(stopVideoButton)(ctx)
			if err == nil {
				testing.ContextLog(ctx, "Close Video")
				return nil
			}
			err = ui.ImmediateLeftClick(startVideoButton)(ctx)
			if err == nil {
				testing.ContextLog(ctx, "Open Video")
				return nil
			}
			return err
		}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
			return err
		}
		return nil
	}

	toggleAudio := func(ctx context.Context) error {
		const (
			muteAudio   = "mute my microphone"
			unmuteAudio = "unmute my microphone"
		)
		ui := uiauto.New(conf.tconn)
		muteAudioButton := nodewith.Name(muteAudio).Role(role.Button)
		unmuteAudioButton := nodewith.Name(unmuteAudio).Role(role.Button)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			err := ui.ImmediateLeftClick(muteAudioButton)(ctx)
			if err == nil {
				testing.ContextLog(ctx, "Mute Audio")
				return nil
			}
			err = ui.ImmediateLeftClick(unmuteAudioButton)(ctx)
			if err == nil {
				testing.ContextLog(ctx, "Open Audio")
				return nil
			}
			return err
		}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
			return err
		}
		return nil
	}

	testing.ContextLog(ctx, "Toggle Video")
	if err := toggleVideo(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	testing.ContextLog(ctx, "Toggle Video")
	if err := toggleVideo(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}
	if err := testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}

	return nil
}

// SwitchTabs switches the chrome tabs.
func (conf *ZoomConference) SwitchTabs(ctx context.Context) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Open wiki page")
	const wikiURL = "https://www.wikipedia.org/"
	wikiConn, err := conf.cr.NewConn(ctx, wikiURL)
	if err != nil {
		return errors.Wrap(err, "failed to open the wiki url")
	}
	defer wikiConn.Close()

	// switch tab
	if err := kb.Accel(ctx, "Ctrl+Tab"); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}

	return nil
}

// ChangeLayout changes the conference UI layout.
func (conf *ZoomConference) ChangeLayout(ctx context.Context) error {
	const (
		view    = "View"
		speaker = "Speaker View"
		gallery = "Gallery View"
	)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()
	ui := uiauto.New(conf.tconn)
	viewButton := nodewith.Name(view).First()
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)

	if err := uiauto.Combine("check view button",
		ui.LeftClick(webArea),
		ui.WaitUntilExists(viewButton),
	)(ctx); err != nil {
		// Some DUTs don't show 'View' button
		testing.ContextLog(ctx, "This DUT doesn't show View button and layout will not be changed")
		return nil
	}

	for _, mode := range []string{speaker, gallery} {
		modeNode := nodewith.Name(mode).Role(role.MenuItem)
		if err := uiauto.Combine("Change layout to '"+mode+"'",
			ui.LeftClick(webArea),
			ui.LeftClick(viewButton),
			ui.LeftClick(modeNode),
			ui.Sleep(10*time.Second), //After applying new layout, give it 10 seconds for viewing before applying next one.
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// BackgroundBlurring blurs the background.
func (conf *ZoomConference) BackgroundBlurring(ctx context.Context) error {
	// Zoom doesn't support background change in web. The common conference test will call this interface
	// and return nil to make sure the test logic passes for zoom.
	// TODO: Add detailed implementation when this feature is available in zoom web.
	return nil
}

// ExtendedDisplayPresenting presents the screen on dextended display.
func (conf *ZoomConference) ExtendedDisplayPresenting(_ context.Context) error {
	// Not required by test case yet.
	return errors.New("extended display presenting for zoom is not implemented")
}

// PresentSlide presents the slides to the conference.
func (conf *ZoomConference) PresentSlide(ctx context.Context) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	shareScreen := func(ctx context.Context) error {
		webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)
		shareScreenButton := nodewith.Name("Share Screen").Role(role.Button)
		presentWindow := nodewith.ClassName("DesktopMediaSourceView").First()
		shareButton := nodewith.Name("Share").Role(role.Button)
		stopShareButton := nodewith.Name("Stop Share").Role(role.Button)

		return uiauto.Combine("Share Screen",
			ui.LeftClick(webArea),
			ui.LeftClickUntil(shareScreenButton, ui.WaitUntilExists(presentWindow)),
			ui.LeftClick(presentWindow),
			ui.LeftClick(shareButton),
			ui.WaitUntilExists(stopShareButton),
		)(ctx)
	}

	testing.ContextLog(ctx, "Create a new Google Slides")
	if err := newGoogleSlides(ctx, conf.cr, false); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch back to conference page")
	if err := conf.switchToChromeTab(ctx, "Zoom"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}

	testing.ContextLog(ctx, "Start to share screen")
	if err := shareScreen(ctx); err != nil {
		return errors.Wrap(err, "failed to share screen")
	}

	testing.ContextLog(ctx, "Switch to the slide")
	if err := conf.switchToChromeTab(ctx, "Untitled presentation - Google Slides"); err != nil {
		return errors.Wrap(err, "failed to switch tab to slide page")
	}

	testing.ContextLog(ctx, "Start present slide")
	if err := presentSlide(ctx, tconn, kb); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Edit slide")
	if err := editSlide(ctx, tconn, kb); err != nil {
		return errors.Wrap(err, "failed to edit slide when leave presentation mode")
	}

	testing.ContextLog(ctx, "Switch back to conference page")
	if err := conf.switchToChromeTab(ctx, "Zoom"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}
	return nil
}

// StopPresenting stops the presentation mode.
func (conf *ZoomConference) StopPresenting(ctx context.Context) error {
	ui := uiauto.New(conf.tconn)
	stopShareButton := nodewith.Name("Stop Share").Role(role.Button)
	testing.ContextLog(ctx, "Stop share")
	return ui.LeftClickUntil(stopShareButton, ui.Gone(stopShareButton))(ctx)
}

// End ends the conference.
func (conf *ZoomConference) End(ctx context.Context) error {
	return cuj.CloseAllWindows(ctx, conf.tconn)
}

var _ Conference = (*ZoomConference)(nil)

// switchToChromeTab switch to the given chrome tab.
//
// TODO: Merge to cuj.UIActionHandler and introduce UIActionHandler in this test. See
// https://chromium-review.googlesource.com/c/chromiumos/platform/tast-tests/+/2779315/
func (conf *ZoomConference) switchToChromeTab(ctx context.Context, tabName string) error {
	ui := uiauto.New(conf.tconn)
	if conf.tabletMode {
		// If in tablet mode, it should toggle tab strip to show tab list.
		if err := ui.LeftClick(nodewith.NameContaining("toggle tab strip").Role(role.Button).First())(ctx); err != nil {
			return err
		}
	}
	return ui.LeftClick(nodewith.NameContaining(tabName).Role(role.Tab))(ctx)
}

// NewZoomConference creates Zoom conference room instance which implements Conference interface.
func NewZoomConference(cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode bool, account string) *ZoomConference {
	return &ZoomConference{cr: cr, tconn: tconn, tabletMode: tabletMode, account: account}
}
