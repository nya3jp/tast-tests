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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// GoogleMeetConference implements the Conference interface.
type GoogleMeetConference struct {
	cr         *chrome.Chrome
	tconn      *chrome.TestConn
	tabletMode bool
	roomSize   int
	account    string
	password   string
}

// Join joins a new conference room.
func (conf *GoogleMeetConference) Join(ctx context.Context, room string) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	meetAccount := conf.account

	//  allowPerm allows camera, microphone and notification if browser asks for the permissions.
	allowPerm := func(ctx context.Context) error {
		allowButton := nodewith.Name("Allow").Role(role.Button)
		dismissButton := nodewith.Name("Dismiss").Role(role.Button)
		avPerm := nodewith.NameRegex(regexp.MustCompile("Use your (microphone|camera)")).ClassName("Label").Role(role.StaticText).First()
		notiPerm := nodewith.Name("Show notifications").ClassName("Label").Role(role.StaticText)

		for _, step := range []struct {
			name   string
			finder *nodewith.Finder
			button *nodewith.Finder
		}{
			{"dismiss permission prompt", dismissButton, dismissButton},
			{"allow microphone and camera", avPerm, allowButton},
			{"allow notifications", notiPerm, allowButton},
		} {
			if err := ui.WithTimeout(4 * time.Second).WaitUntilExists(step.finder)(ctx); err == nil {
				if err := uiauto.Combine(step.name, ui.LeftClick(step.button), ui.Sleep(2*time.Second))(ctx); err != nil {
					return err
				}
			} else {
				testing.ContextLog(ctx, "No action is required to ", step.name)
			}
		}
		return nil
	}

	joinConf := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Join Conference")
		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		if err := uiauto.Combine("Join conference",
			ui.WithTimeout(40*time.Second).LeftClickUntil(joinNowButton, ui.WaitUntilGone(nodewith.Name("Return to home screen"))),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to join conference")
		}
		return nil
	}

	// Using existed conference-test account for Google Meet testing,
	// and add the test account if it doesn't add in the DUT before.
	addMeetAccount := func(ctx context.Context) error {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to initialize keyboard input")
		}
		defer kb.Close()

		useAnotherAccount := nodewith.Name("Use another account").First()
		myAccounts := nodewith.Name("My accounts").First()
		viewAccounts := nodewith.Name("View accounts").Role(role.Button)

		if err := ui.LeftClick(useAnotherAccount)(ctx); err != nil {
			return err
		}

		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(viewAccounts)(ctx); err == nil {
			if err := ui.LeftClick(viewAccounts)(ctx); err != nil {
				return err
			}
		}

		if err := ui.WaitUntilExists(myAccounts)(ctx); err != nil {
			return err
		}

		addAccountButton := nodewith.Name("Add account").Role(role.Button)
		emailField := nodewith.Name("Email or phone").Role(role.TextField)
		nextButton := nodewith.Name("Next").Role(role.Button)
		passwordField := nodewith.Name("Enter your password").Role(role.TextField)
		iAgree := nodewith.Name("I agree").Role(role.Button)
		if err := uiauto.Combine("Add account",
			ui.LeftClick(addAccountButton),
			ui.LeftClick(emailField),
			kb.TypeAction(meetAccount),
			ui.LeftClick(nextButton),
			ui.LeftClick(passwordField),
			kb.TypeAction(conf.password),
			ui.LeftClick(nextButton),
			ui.LeftClick(iAgree),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to add account")
		}

		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			return errors.Wrap(err, "failed to close settings page")
		}

		chooseAnAccount := nodewith.Name("Choose an account").First()
		if err := ui.WaitUntilExists(chooseAnAccount)(ctx); err != nil {
			return errors.Wrap(err, "failed to find 'Choose an account'")
		}
		return nil
	}

	// If gaia-login isn't use the conference-test only account, it would switch
	// when running the case. And also add the test account if the DUT doesn't
	// be added before.
	switchUserJoin := func(ctx context.Context) error {
		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		if err := ui.WithTimeout(3 * time.Second).Exists(joinNowButton); err == nil {
			testing.ContextLog(ctx, "Join the meeting without switching account")
			return nil
		}
		testing.ContextLog(ctx, "Switch account")
		switchAccount := nodewith.Name("Switch account").Role(role.Link)
		meetAccountText := nodewith.Name(meetAccount).First()
		chooseAnAccount := nodewith.Name("Choose an account").First()
		if err := uiauto.Combine("switch account",
			ui.LeftClickUntil(switchAccount, ui.Gone(switchAccount)),
			ui.WaitUntilExists(chooseAnAccount),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch account")
		}

		if err := ui.WaitUntilExists(meetAccountText)(ctx); err != nil {
			testing.ContextLogf(ctx, "Add additional account %s to existing account", meetAccount)
			if err := addMeetAccount(ctx); err != nil {
				return errors.Wrapf(err, "failed to add account %s", meetAccount)
			}
		}
		testing.ContextLog(ctx, "Select meet account ", meetAccount)

		if err := uiauto.Combine("select account",
			ui.WaitUntilExists(meetAccountText),
			ui.WithTimeout(time.Minute).LeftClickUntil(meetAccountText, ui.WaitUntilExists(joinNowButton)),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s and wait for Join Now button", meetAccount)
		}
		return nil
	}

	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size
	checkParticipantsNum := func(ctx context.Context) error {
		showEveryone := nodewith.Name("Show everyone").Role(role.Button)
		people := nodewith.Name("People").First()
		// Some DUT models have poor performance. When joining
		// a large conference (over 15 participants), it would take much time
		// to render DOM elements. Set a longer timer here.
		if err := uiauto.Combine("Click 'Show everyone'",
			ui.WithTimeout(2*time.Minute).WaitUntilExists(showEveryone),
			ui.LeftClick(showEveryone),
			ui.WithTimeout(30*time.Second).WaitUntilExists(people),
		)(ctx); err != nil {
			return err
		}
		participant := nodewith.NameRegex(regexp.MustCompile("[0-9]+ participant[s]?")).Role(role.Tab).First()
		participantInfo, err := ui.Info(ctx, participant)
		if err != nil {
			return errors.Wrap(err, "failed to get participant info")
		}
		strs := strings.Split(participantInfo.Name, " ")
		num, err := strconv.ParseInt(strs[0], 10, 64)
		if err != nil {
			return errors.Wrap(err, "cannot parse number of participants")
		}
		if int(num) != conf.roomSize {
			return errors.Wrapf(err, "meeting participant number is %d but %d is expected", num, conf.roomSize)
		}
		return nil
	}

	if _, err := conf.cr.NewConn(ctx, room); err != nil {
		return errors.Wrap(err, "failed to create participant join conference")
	}

	if err := allowPerm(ctx); err != nil {
		return err
	}

	if err := switchUserJoin(ctx); err != nil {
		return err
	}

	if err := joinConf(ctx); err != nil {
		return err
	}

	if err := checkParticipantsNum(ctx); err != nil {
		return errors.Wrap(err, "failed to check participants number")
	}

	return nil
}

// VideoAudioControl controls the video and audio during conference.
func (conf *GoogleMeetConference) VideoAudioControl(ctx context.Context) error {
	// It may take some time to detect the microphone or camera button from the meet UI.
	const detectButtonTime = 30 * time.Second

	toggleVideo := func(ctx context.Context) error {
		ui := uiauto.New(conf.tconn)
		cameraButton := nodewith.NameRegex(regexp.MustCompile("Turn (on|off) camera.*")).Role(role.Button)

		info, err := ui.WithTimeout(detectButtonTime).Info(ctx, cameraButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet camera switch button to show")
		}
		if strings.HasPrefix(info.Name, "Turn on") {
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
		ui := uiauto.New(conf.tconn)
		microphoneButton := nodewith.NameRegex(regexp.MustCompile("Turn (on|off) microphone.*")).Role(role.Button)

		info, err := ui.WithTimeout(detectButtonTime).Info(ctx, microphoneButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet microphone switch button to show")
		}
		if strings.HasPrefix(info.Name, "Turn on") {
			testing.ContextLog(ctx, "Turn microphone from off to on")
		} else {
			testing.ContextLog(ctx, "Turn microphone from on to off")
		}
		if err := ui.LeftClick(microphoneButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch microphone")
		}
		return nil
	}

	for _, f := range []func(ctx context.Context) error{
		toggleVideo, toggleVideo, toggleAudio, toggleAudio,
	} {
		if err := f(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle video or audio switch")
		}
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
	}
	return nil
}

// SwitchTabs switches the chrome tabs.
func (conf *GoogleMeetConference) SwitchTabs(ctx context.Context) error {
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
func (conf *GoogleMeetConference) ChangeLayout(ctx context.Context) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)

	ns, err := ash.Notifications(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get notifications")
	}
	if len(ns) > 0 {
		testing.ContextLog(ctx, "Hide visible notifications")
		// Hide notifications which could cover the "More options" button.
		if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to hide notifications")
		}
		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep after hiding notifications")
		}
	}

	moreOptions := nodewith.Name("More options").First()
	changeLayout := nodewith.Name("Change layout").Role(role.MenuItem)
	for _, mode := range []string{"Tiled", "Spotlight"} {
		modeNode := nodewith.Name(mode).Role(role.RadioButton)
		testing.ContextLog(ctx, "Change layout to ", mode)
		if err := uiauto.Combine("change layout",
			ui.WithTimeout(40*time.Second).LeftClick(moreOptions),
			ui.WithTimeout(40*time.Second).LeftClick(changeLayout),
			ui.LeftClick(modeNode),
			ui.LeftClick(moreOptions),
			ui.Sleep(10*time.Second), //After applying new layout, give it 10 seconds for viewing before applying next one.
		)(ctx); err != nil {
			return err
		}
	}

	return nil
}

// BackgroundBlurring blurs the background.
func (conf *GoogleMeetConference) BackgroundBlurring(ctx context.Context) error {
	const (
		blurBackground    = "Blur your background"
		skyBackground     = "Blurry sky with purple horizon background"
		turnOffBackground = "Turn off background effects"
	)
	ui := uiauto.New(conf.tconn)
	changeBackground := func(background string) error {
		moreOptions := nodewith.Name("More options").First()
		changeBackground := nodewith.Name("Change background").Role(role.MenuItem)
		backgroundButton := nodewith.Name(background).First()
		video := nodewith.Role(role.Video).First()
		return uiauto.Combine("Change background",
			ui.LeftClick(moreOptions),
			ui.LeftClick(changeBackground),
			ui.WithTimeout(30*time.Second).LeftClick(backgroundButton),
			ui.LeftClick(video), // close background menu.
			ui.Sleep(5*time.Second),
		)(ctx)
	}

	if err := changeBackground(blurBackground); err != nil {
		return errors.Wrap(err, "failed to change background to blur background")
	}

	if err := ui.LeftClick(nodewith.Name("Pin yourself to your main screen."))(ctx); err != nil {
		return errors.Wrap(err, "failed to switch to your main screen")
	}

	if err := changeBackground(skyBackground); err != nil {
		return errors.Wrap(err, "failed to change background to sky background")
	}

	if err := changeBackground(turnOffBackground); err != nil {
		return errors.Wrap(err, "failed to turn off background effects")
	}

	return nil
}

// PresentSlide presents the slides to the conference.
func (conf *GoogleMeetConference) PresentSlide(ctx context.Context) error {
	const slideTitle = "Untitled presentation - Google Slides"
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	// Make Google Meet to show the bottom bar
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	shareScreen := func(ctx context.Context, tconn *chrome.TestConn) error {
		presentNowButton := nodewith.Name("Present now").First()
		aWindow := nodewith.Name("A window").Role(role.MenuItem)
		presentWindow := nodewith.ClassName("DesktopMediaSourceView").First()
		shareButton := nodewith.Name("Share").Role(role.Button)
		stopPresenting := nodewith.Name("Stop presenting").Role(role.Button)
		return uiauto.Combine("Share screen",
			ui.LeftClick(presentNowButton),
			ui.WithTimeout(time.Minute).LeftClickUntil(aWindow, ui.WaitUntilExists(presentWindow)),
			ui.LeftClick(presentWindow),
			ui.LeftClickUntil(shareButton, ui.Gone(shareButton)),
			ui.WithTimeout(2*time.Minute).WaitUntilExists(stopPresenting),
		)(ctx)
	}

	testing.ContextLog(ctx, "Create a new Google Slides")
	if err := newGoogleSlides(ctx, conf.cr, false); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch to conference page")
	if err := conf.switchToChromeTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}

	testing.ContextLog(ctx, "Click present now button")
	if err := shareScreen(ctx, tconn); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch to the slide")
	if err := conf.switchToChromeTab(ctx, slideTitle); err != nil {
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

	testing.ContextLog(ctx, "Switch to conference page")
	if err := conf.switchToChromeTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}

	return nil
}

// ExtendedDisplayPresenting presents the screen on dextended display.
func (conf *GoogleMeetConference) ExtendedDisplayPresenting(ctx context.Context) error {
	const slideTitle = "Untitled presentation - Google Slides"
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	// Make Google Meet to show the bottom bar
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	unsetMirrorDisplay := func(ctx context.Context, tconn *chrome.TestConn) error {
		testing.ContextLog(ctx, "Launch os settins to disable mirror")
		settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Device").Role(role.Link))
		if err != nil {
			return errors.Wrap(err, "failed to launch os-settings Device page")
		}

		displayFinder := nodewith.Name("Displays").Role(role.Link).Ancestor(ossettings.WindowFinder)
		if err := ui.LeftClick(displayFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to launch display page")
		}

		mirrorFinder := nodewith.Name("Mirror Built-in display").Role(role.CheckBox).Ancestor(ossettings.WindowFinder)
		if err := ui.LeftClick(mirrorFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}

		if err := settings.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close settings")
		}

		return nil
	}

	moveConferenceTab := func(ctx context.Context) error {
		return uiauto.Combine("Move conference to exteneded display",
			kb.AccelAction("Alt+Tab"),
			ui.Sleep(400*time.Millisecond),
			kb.AccelAction("Search+Alt+M"),
			ui.Sleep(400*time.Millisecond),
		)(ctx)
	}

	enterPresentingMode := func(ctx context.Context) error {
		presentNowButton := nodewith.Name("Present now").First()
		aWindow := nodewith.Name("A window").First()
		return uiauto.Combine("Share Screen",
			kb.AccelAction("Alt+Tab"),
			ui.LeftClick(presentNowButton),
			ui.LeftClick(aWindow),
		)(ctx)
	}

	selectSharedWindow := func(ctx context.Context) error {
		presentWindow := nodewith.ClassName("DesktopMediaSourceView").NameRegex(regexp.MustCompile("My Drive"))
		shareButton := nodewith.Name("Share").Role(role.Button)
		stopPresentation := nodewith.Name("Stop presenting").Role(role.Button)
		return uiauto.Combine("Select Shared Tab",
			ui.LeftClick(presentWindow),
			ui.LeftClickUntil(shareButton, ui.Gone(shareButton)),
			ui.WaitUntilGone(stopPresentation),
		)(ctx)
	}

	if conf.tabletMode {
		if err := unsetMirrorDisplay(ctx, tconn); err != nil {
			return err
		}
	}

	testing.ContextLog(ctx, "Create a new Google Slides")
	if err := newGoogleSlides(ctx, conf.cr, true); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch to conference page")
	if err := conf.switchToChromeTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}

	testing.ContextLog(ctx, "Enter Presenting Mode")
	if err := enterPresentingMode(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Select Window for Sharing")
	if err := selectSharedWindow(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Move conference tab to extended display")
	if err := moveConferenceTab(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch to the slide")
	if err := conf.switchToChromeTab(ctx, slideTitle); err != nil {
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

	testing.ContextLog(ctx, "Switch to conference page")
	if err := conf.switchToChromeTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}
	return nil
}

// StopPresenting stops the presentation mode.
func (conf *GoogleMeetConference) StopPresenting(ctx context.Context) error {
	ui := uiauto.New(conf.tconn)
	stopPresentingButton := nodewith.Name("Stop presenting").Role(role.Button)
	testing.ContextLog(ctx, "Stop presenting")
	return ui.LeftClickUntil(stopPresentingButton, ui.WithTimeout(3*time.Second).WaitUntilGone(stopPresentingButton))(ctx)
}

// End ends the conference.
func (conf *GoogleMeetConference) End(ctx context.Context) error {
	cuj.CloseAllWindows(ctx, conf.tconn)
	return nil
}

var _ Conference = (*GoogleMeetConference)(nil)

// switchToChromeTab switch to the given chrome tab.
//
// TODO: Merge to cuj.UIActionHandler and introduce UIActionHandler in this test. See
// https://chromium-review.googlesource.com/c/chromiumos/platform/tast-tests/+/2779315/
func (conf *GoogleMeetConference) switchToChromeTab(ctx context.Context, tabName string) error {
	ui := uiauto.New(conf.tconn)
	if conf.tabletMode {
		// If in tablet mode, it should toggle tab strip to show tab list.
		toggle := nodewith.NameRegex(regexp.MustCompile("toggle tab strip")).Role(role.Button).First()
		if err := ui.Exists(toggle)(ctx); err == nil {
			if err := ui.LeftClick(toggle)(ctx); err != nil {
				return err
			}
		}
	}
	tab := nodewith.NameRegex(regexp.MustCompile(tabName)).Role(role.Tab)
	return uiauto.Combine("Click tab",
		ui.WaitUntilExists(tab),
		ui.LeftClick(tab),
	)(ctx)
}

// NewGoogleMeetConference creates Google Meet conference room instance which implements Conference interface.
func NewGoogleMeetConference(cr *chrome.Chrome, tconn *chrome.TestConn, tabletMode bool,
	roomSize int, account, password string) *GoogleMeetConference {
	return &GoogleMeetConference{
		cr:         cr,
		tconn:      tconn,
		tabletMode: tabletMode,
		roomSize:   roomSize,
		account:    account,
		password:   password,
	}
}
