// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
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
	roomSize   int
	account    string
}

// Join joins a new conference room.
func (conf *ZoomConference) Join(ctx context.Context, room string) error {
	ui := uiauto.New(conf.tconn)
	openZoomAndSignIn := func(ctx context.Context) error {
		const zoomURL = "https://zoom.us/"
		conn, err := conf.cr.NewConn(ctx, zoomURL)
		if err != nil {
			return errors.Wrap(err, "failed to open the zoom website")
		}
		defer conn.Close()

		if err := webutil.WaitForQuiescence(ctx, conn, 45*time.Second); err != nil {
			return errors.Wrapf(err, "failed to wait for %q to be loaded and achieve quiescence", room)
		}

		if err := ui.WaitUntilExists(nodewith.Name("SIGN IN").Role(role.Link))(ctx); err == nil {
			testing.ContextLog(ctx, "Start to sign in")
			if err := conn.Navigate(ctx, "https://zoom.us/google_oauth_signin"); err != nil {
				return err
			}
			account := nodewith.Name(conf.account).First()
			profilePicture := nodewith.Name("Profile picture").First()
			// If the DUT has only one account, it would login to profile page directly.
			// Otherwise, it would show list of accounts.
			if err := uiauto.Combine("sign in",
				ui.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(account),
					ui.LeftClickUntil(account, ui.Gone(account))),
				ui.WaitUntilExists(profilePicture),
			)(ctx); err != nil {
				return err
			}
		} else {
			testing.ContextLog(ctx, "It has been signed in")
		}
		if err := conn.Navigate(ctx, room); err != nil {
			return err
		}
		return nil
	}

	//  allowPerm allows camera, microphone and notification if browser asks for the permissions.
	allowPerm := func(ctx context.Context) error {
		allowButton := nodewith.Name("Allow").Role(role.Button)
		cameraPerm := nodewith.NameRegex(regexp.MustCompile("Use your camera")).ClassName("RootView").Role(role.AlertDialog).First()
		microphonePerm := nodewith.NameRegex(regexp.MustCompile("Use your microphone")).ClassName("RootView").Role(role.AlertDialog).First()
		notiPerm := nodewith.NameContaining("Show notifications").ClassName("RootView").Role(role.AlertDialog)

		for _, step := range []struct {
			name   string
			finder *nodewith.Finder
			button *nodewith.Finder
		}{
			{"allow notifications", notiPerm, allowButton.Ancestor(notiPerm)},
			{"allow microphone", microphonePerm, allowButton.Ancestor(microphonePerm)},
			{"allow camera", cameraPerm, allowButton.Ancestor(cameraPerm)},
		} {
			if err := ui.WithTimeout(4 * time.Second).WaitUntilExists(step.finder)(ctx); err == nil {
				// Immediately clicking the allow button sometimes doesn't work. Sleep 2 seconds.
				if err := uiauto.Combine(step.name, ui.Sleep(2*time.Second), ui.LeftClick(step.button), ui.WaitUntilGone(step.finder))(ctx); err != nil {
					return err
				}
			} else {
				testing.ContextLog(ctx, "No action is required to ", step.name)
			}
		}
		return nil
	}

	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size
	checkParticipantsNum := func(ctx context.Context) error {
		participant := nodewith.NameContaining("open the participants list pane").Role(role.Button)
		participantInfo, err := ui.Info(ctx, participant)
		if err != nil {
			return errors.Wrap(err, "failed to get participant info")
		}
		testing.ContextLog(ctx, "Get participant info: ", participantInfo.Name)
		strs := strings.Split(participantInfo.Name, "[")
		strs = strings.Split(strs[1], "]")
		num, err := strconv.ParseInt(strs[0], 10, 64)
		if err != nil {
			return errors.Wrap(err, "cannot parse number of participants")
		}
		if int(num) != conf.roomSize {
			return errors.Wrapf(err, "meeting participant number is %d but %d is expected", num, conf.roomSize)
		}
		return nil
	}

	startVideo := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start video")
		cameraButton := nodewith.NameRegex(regexp.MustCompile("(stop|start) sending my video")).Role(role.Button)
		startVideoButton := nodewith.Name("start sending my video").Role(role.Button)
		stopVideoButton := nodewith.Name("stop sending my video").Role(role.Button)
		webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)

		// Some DUTs start playing video for the first time.
		// If there is a stop video button, do nothing.
		return uiauto.Combine("start video",
			allowPerm,
			// Click web area in order to make the camera button reappear.
			ui.IfSuccessThen(ui.Gone(cameraButton), ui.LeftClick(webArea)),
			ui.WaitUntilExists(cameraButton),
			ui.IfSuccessThen(ui.Exists(startVideoButton),
				ui.LeftClickUntil(startVideoButton, ui.WithTimeout(time.Second).WaitUntilGone(startVideoButton))),
			ui.WaitUntilExists(stopVideoButton),
		)(ctx)
	}

	testing.ContextLog(ctx, "Join conference")
	joinFromYourBrowser := nodewith.Name("Join from Your Browser").Role(role.StaticText)
	joinButton := nodewith.Name("Join").Role(role.Button)
	joinAudioButton := nodewith.Name("Join Audio by Computer").Role(role.Button)

	// It seems zoom has different UI versions. One of the zoom version will open a new tab.
	// Need to close the initial zoom web page to avoid problems when switching tabs.
	closeLaunchMeetingTab := func(ctx context.Context) error {
		zoomTab := nodewith.Name("Launch Meeting - Zoom").Role(role.Tab)
		closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(zoomTab)
		if conf.tabletMode {
			// If in tablet mode, it should toggle tab strip to show tab list.
			if err := ui.LeftClick(nodewith.NameContaining("toggle tab strip").Role(role.Button).First())(ctx); err != nil {
				return err
			}
		}
		if err := ui.LeftClick(closeButton)(ctx); err == nil {
			testing.ContextLog(ctx, `Close "Launch Meeting - Zoom" tab`)
		}
		return nil
	}
	return uiauto.Combine("join conference",
		openZoomAndSignIn,
		ui.LeftClick(joinFromYourBrowser),
		ui.WithTimeout(time.Minute).WaitUntilExists(joinButton),
		ui.LeftClickUntil(joinButton, ui.WithTimeout(1*time.Second).WaitUntilGone(joinButton)),
		ui.WithTimeout(30*time.Second).WaitUntilExists(joinAudioButton),
		checkParticipantsNum,
		ui.LeftClickUntil(joinAudioButton, ui.WithTimeout(time.Second).WaitUntilGone(joinAudioButton)),
		// Launch Meeting page is useless so close it.
		closeLaunchMeetingTab,
		// Start video requires camera permission.
		// Allow permission doesn't succeed every time. So add retry here.
		ui.Retry(3, startVideo),
	)(ctx)
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
	const slideTitle = "Untitled presentation - Google Slides"
	tconn := conf.tconn
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	shareScreen := func(ctx context.Context) error {
		shareScreenButton := nodewith.Name("Share Screen").Role(role.Button)
		presentWindow := nodewith.ClassName("DesktopMediaSourceView").First()
		shareButton := nodewith.Name("Share").Role(role.Button)
		stopShareButton := nodewith.Name("Stop Share").Role(role.Button)
		testing.ContextLog(ctx, "Start to share screen")
		return uiauto.Combine("share Screen",
			conf.showInterface,
			ui.LeftClickUntil(shareScreenButton, ui.WithTimeout(time.Second).WaitUntilExists(presentWindow)),
			ui.LeftClick(presentWindow),
			ui.LeftClick(shareButton),
			ui.WaitUntilExists(stopShareButton),
		)(ctx)
	}

	deletePerformed := false
	// slideCleanup switches to the slide page and delete it.
	slideCleanup := func(ctx context.Context) error {
		if deletePerformed {
			return nil
		}
		deletePerformed = true // Set it to true because we only try to delete once.
		testing.ContextLog(ctx, "Switch to the slide page and delete it")
		return uiauto.Combine("switch to the slide page and delete it",
			conf.switchToChromeTab(slideTitle),
			deleteSlide(conf.tconn),
		)(ctx)
	}

	// Shorten the context to cleanup slide.
	cleanUpSlideCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	testing.ContextLog(ctx, "Create a new Google Slides")
	if err := newGoogleSlides(ctx, conf.cr, false); err != nil {
		return err
	}
	defer func() {
		if err := slideCleanup(cleanUpSlideCtx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
		}
	}()

	return uiauto.Combine("present slide",
		conf.switchToChromeTab("Zoom"),
		shareScreen,
		conf.switchToChromeTab(slideTitle),
		presentSlide(tconn, kb),
		editSlide(tconn, kb),
		func(ctx context.Context) error {
			if err := slideCleanup(ctx); err != nil {
				// Only log the error.
				testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
			}
			return nil
		},
		conf.switchToChromeTab("Zoom"),
	)(ctx)
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
func (conf *ZoomConference) switchToChromeTab(tabName string) action.Action {
	ui := uiauto.New(conf.tconn)
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switch tab to %q", tabName)
		if conf.tabletMode {
			// If in tablet mode, it should toggle tab strip to show tab list.
			if err := ui.LeftClick(nodewith.NameContaining("toggle tab strip").Role(role.Button).First())(ctx); err != nil {
				return err
			}
		}
		if err := ui.LeftClick(nodewith.NameContaining(tabName).Role(role.Tab))(ctx); err != nil {
			return errors.Wrapf(err, "failed to switch tab to %q", tabName)
		}
		return nil
	}
}

// NewZoomConference creates Zoom conference room instance which implements Conference interface.
func NewZoomConference(cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode bool,
	roomSize int, account string) *ZoomConference {
	return &ZoomConference{
		cr:         cr,
		tconn:      tconn,
		tabletMode: tabletMode,
		roomSize:   roomSize,
		account:    account,
	}
}
