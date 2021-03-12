// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
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

	if err := webutil.WaitForQuiescence(ctx, conn, 45*time.Second); err != nil {
		return errors.Wrapf(err, "failed to wait for %q to be loaded and achieve quiescence", room)
	}

	testing.ContextLog(ctx, "Click 'Join from Your Browser' link")
	joinFromYourBrowser := nodewith.Name("Join from Your Browser").Role(role.Anchor)
	joinButton := nodewith.Name("Join").Role(role.Button)
	if err := uiauto.Combine("join from Your Browser",
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
	//  allowPerm allows camera, microphone if browser asks for the permissions.
	allowPerm := func(ctx context.Context) error {
		stepName := "allow microphone and camera"
		avPerm := nodewith.NameRegex(regexp.MustCompile(".*Use your (microphone|camera).*")).ClassName("RootView").Role(role.AlertDialog).First()
		allowButton := nodewith.Name("Allow").Role(role.Button).Ancestor(avPerm)

		if err := ui.WithTimeout(4 * time.Second).WaitUntilExists(avPerm)(ctx); err == nil {
			// Immediately clicking the allow button sometimes doesn't work. Sleep 2 seconds.
			if err := uiauto.Combine(stepName,
				ui.Sleep(2*time.Second),
				ui.LeftClick(allowButton),
				ui.WaitUntilGone(avPerm))(ctx); err != nil {
				return err
			}
		} else {
			testing.ContextLog(ctx, "No action is required to ", stepName)
		}
		return nil
	}

	if err := ui.WaitUntilExists(joinButton)(ctx); err != nil {
		return err
	}

	if err := allowPerm(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Join conference")
	joinAudioButton := nodewith.Name("Join Audio by Computer").Role(role.Button)
	if err := uiauto.Combine("join conference",
		ui.LeftClick(joinButton),
		ui.WithTimeout(time.Second*30).WaitUntilExists(joinAudioButton),
		ui.LeftClickUntil(joinAudioButton, ui.Gone(joinAudioButton)),
	)(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Start video")
	startVideoButton := nodewith.Name("start sending my video").Role(role.Button)
	stopVideoButton := nodewith.Name("stop sending my video").Role(role.Button)
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)

	// If kept idle, the zoom UI buttons may become invisible.
	// Click web area in order to make the video button reappear.
	if err := ui.LeftClick(webArea)(ctx); err != nil {
		return err
	}

	cameraButton := nodewith.NameRegex(regexp.MustCompile("(stop|start) sending my video")).Role(role.Button)
	info, err := ui.Info(ctx, cameraButton)
	if err != nil {
		return errors.Wrap(err, "failed to wait for the meet camera switch button to show")
	}
	// Some DUTs start playing video for the first time.
	// If there is a stop video button, do nothing.
	if strings.HasPrefix(info.Name, "start") {
		if err := uiauto.Combine("start video",
			ui.LeftClick(startVideoButton),
			ui.WaitUntilExists(stopVideoButton),
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// VideoAudioControl controls the video and audio during conference.
func (conf *ZoomConference) VideoAudioControl(ctx context.Context) error {
	ui := uiauto.New(conf.tconn)
	toggleVideo := func(ctx context.Context) error {
		cameraButton := nodewith.NameRegex(regexp.MustCompile("(stop|start) sending my video")).Role(role.Button)

		info, err := ui.Info(ctx, cameraButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet camera switch button to show")
		}
		if strings.HasPrefix(info.Name, "start") {
			testing.ContextLog(ctx, "Turn camera from off to on")
		} else {
			testing.ContextLog(ctx, "Turn camera from on to off")
		}
		if err := ui.LeftClick(cameraButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch camera")
		}
		return nil
	}

	toggleAudio := func(ctx context.Context) error {
		microphoneButton := nodewith.NameRegex(regexp.MustCompile("(mute|unmute) my microphone")).Role(role.Button)

		info, err := ui.Info(ctx, microphoneButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet microphone switch button to show")
		}
		if strings.HasPrefix(info.Name, "unmute") {
			testing.ContextLog(ctx, "Turn microphone from mute to unmute")
		} else {
			testing.ContextLog(ctx, "Turn microphone from unmute to mute")
		}
		if err := ui.LeftClick(microphoneButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch microphone")
		}
		return nil
	}

	return uiauto.Combine("toggle video and audio",
		//Remain in the state for 5 seconds after each action.
		toggleVideo, ui.Sleep(5*time.Second),
		toggleVideo, ui.Sleep(5*time.Second),
		toggleAudio, ui.Sleep(5*time.Second),
		toggleAudio, ui.Sleep(5*time.Second),
	)(ctx)
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
		if err := uiauto.Combine("change layout to '"+mode+"'",
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

		return uiauto.Combine("share Screen",
			ui.LeftClick(webArea),
			ui.LeftClickUntil(shareScreenButton, ui.WithTimeout(time.Second).WaitUntilExists(presentWindow)),
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
