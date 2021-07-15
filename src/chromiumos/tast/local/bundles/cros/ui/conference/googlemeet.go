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
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
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
	const showJoinNowTimeout = time.Minute
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	meetAccount := conf.account

	openConference := func(ctx context.Context) error {
		conn, err := conf.cr.NewConn(ctx, room)
		if err != nil {
			return errors.Wrap(err, "failed to create chrome connection to join the conference")
		}

		if err := webutil.WaitForQuiescence(ctx, conn, 45*time.Second); err != nil {
			return errors.Wrapf(err, "failed to wait for %q to be loaded and achieve quiescence", room)
		}
		return nil
	}

	//  allowPerm allows camera, microphone and notification if browser asks for the permissions.
	allowPerm := func(ctx context.Context) error {
		allowButton := nodewith.Name("Allow").Role(role.Button)
		dismissButton := nodewith.Name("Dismiss").Role(role.Button)
		avPerm := nodewith.NameRegex(regexp.MustCompile(".*Use your (microphone|camera).*")).ClassName("RootView").Role(role.AlertDialog).First()
		notiPerm := nodewith.NameContaining("Show notifications").ClassName("RootView").Role(role.AlertDialog)

		for _, step := range []struct {
			name   string
			finder *nodewith.Finder
			button *nodewith.Finder
		}{
			{"dismiss permission prompt", dismissButton, dismissButton},
			{"allow microphone and camera", avPerm, allowButton.Ancestor(avPerm)},
			{"allow notifications", notiPerm, allowButton.Ancestor(notiPerm)},
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

	joinConf := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Join Conference")
		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		homeScreenLink := nodewith.Name("Return to home screen").Role(role.Link)
		if err := ui.WithTimeout(showJoinNowTimeout).LeftClickUntil(joinNowButton, ui.WaitUntilGone(homeScreenLink))(ctx); err != nil {
			return errors.Wrapf(err, "failed to click button to join conference within %v", showJoinNowTimeout)
		}
		return nil
	}

	// enterAccount enter account email and password.
	enterAccount := func(ctx context.Context) error {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to initialize keyboard input")
		}
		defer kb.Close()

		emailContent := nodewith.NameContaining(meetAccount).Editable()
		emailField := nodewith.Name("Email or phone").Role(role.TextField)
		emailFieldFocused := nodewith.Name("Email or phone").Role(role.TextField).Focused()
		nextButton := nodewith.Name("Next").Role(role.Button)
		passwordField := nodewith.Name("Enter your password").Role(role.TextField)
		passwordFieldFocused := nodewith.Name("Enter your password").Role(role.TextField).Focused()
		iAgree := nodewith.Name("I agree").Role(role.Button)

		var actions []uiauto.Action
		if err := ui.WaitUntilExists(emailContent)(ctx); err != nil {
			// Email has not been entered into the text box yet.
			actions = append(actions,
				// Make sure text area is focused before typing. This is especially necessary on low-end DUTs.
				ui.LeftClickUntil(emailField, ui.Exists(emailFieldFocused)),
				kb.TypeAction(meetAccount),
			)
		}
		actions = append(actions,
			ui.LeftClick(nextButton),
			// Make sure text area is focused before typing. This is especially necessary on low-end DUTs.
			ui.LeftClickUntil(passwordField, ui.Exists(passwordFieldFocused)),
			kb.TypeAction(conf.password),
			ui.LeftClick(nextButton),
			ui.LeftClickUntil(iAgree, ui.WithTimeout(1*time.Second).WaitUntilGone(iAgree)),
		)
		if err := uiauto.Combine("enter email and password",
			actions...,
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to enter account info")
		}
		return nil
	}

	// Using existed conference-test account for Google Meet testing,
	// and add the test account if it doesn't add in the DUT before.
	addMeetAccount := func(ctx context.Context) error {
		useAnotherAccount := nodewith.Name("Use another account").First()
		if err := ui.LeftClick(useAnotherAccount)(ctx); err != nil {
			return errors.Wrap(err, `failed to click "Use another account"`)
		}

		addAccPrompt := nodewith.NameStartingWith("Add another Google Account for").Role(role.Heading)
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(addAccPrompt)(ctx); err == nil {
			// Close all notifications to prevent them from covering the ok button.
			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to close notifications")
			}

			dontReminder := nodewith.Name("Don't remind me next time").Role(role.CheckBox)
			if err := ui.LeftClick(dontReminder)(ctx); err != nil {
				return errors.Wrap(err, `failed to click "Don't remind me next time"`)
			}

			signInWebArea := nodewith.Name("Sign in to add a Google account").Role(role.RootWebArea)
			okBtn := nodewith.Name("OK").Role(role.Button).Ancestor(signInWebArea)
			if err := ui.LeftClick(okBtn)(ctx); err != nil {
				return errors.Wrap(err, `failed to click "OK" for new account prompt`)
			}
		}

		if err := enterAccount(ctx); err != nil {
			return err
		}

		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			return errors.Wrap(err, "failed to close settings page")
		}

		chooseAnAccount := nodewith.Name("Choose an account").First()
		if err := ui.WaitUntilExists(chooseAnAccount)(ctx); err != nil {
			return errors.Wrap(err, `failed to find "Choose an account"`)
		}
		return nil
	}

	// switchUser switches to the account that will be used to join the Google meet.
	switchUser := func(ctx context.Context) error {
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

		// If meet account doesn't exist, add the account first.
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(meetAccountText)(ctx); err != nil {
			testing.ContextLogf(ctx, "Add additional account %s to existing account", meetAccount)
			if err := addMeetAccount(ctx); err != nil {
				return errors.Wrapf(err, "failed to add account %s", meetAccount)
			}
		}
		testing.ContextLog(ctx, "Select meet account ", meetAccount)

		nextUI := nodewith.NameRegex(regexp.MustCompile("(Join now|Email or phone)")).First()
		if err := uiauto.Combine("select account",
			ui.WaitUntilExists(meetAccountText),
			ui.WithTimeout(showJoinNowTimeout).LeftClickUntil(meetAccountText, ui.WaitUntilExists(nextUI)),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s", meetAccount)
		}

		// Check if signing into the meet account is required.
		emailField := nodewith.Name("Email or phone").Role(role.TextField)
		if err := ui.Exists(emailField)(ctx); err == nil {
			testing.ContextLog(ctx, "Signin is required when switching account")
			if err := enterAccount(ctx); err != nil {
				return errors.Wrapf(err, "failed to enter account %s", meetAccount)
			}
		}

		// Wait for the "Join now" button.
		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		if err := ui.WithTimeout(showJoinNowTimeout).WaitUntilExists(joinNowButton)(ctx); err != nil {
			return errors.Wrapf(err, "Join now button didn't show for account %s", meetAccount)
		}
		return nil
	}

	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size.
	checkParticipantsNum := func(ctx context.Context) error {
		meetWebArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
		participant := nodewith.NameRegex(regexp.MustCompile(`^[\d]+$`)).Role(role.StaticText).Ancestor(meetWebArea)
		// Some DUT models have poor performance. When joining
		// a large conference (over 15 participants), it would take much time
		// to render DOM elements. Set a longer timer here.
		if err := ui.WithTimeout(2 * time.Minute).WaitUntilExists(participant)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait participant info")
		}
		participantInfo, err := ui.Info(ctx, participant)
		if err != nil {
			return errors.Wrap(err, "failed to get participant info")
		}
		strs := strings.Split(participantInfo.Name, " ")
		num, err := strconv.ParseInt(strs[0], 10, 64)
		if err != nil {
			return errors.Wrap(err, "cannot parse number of participants")
		}
		// Check number of participants following this logic:
		// - Class size room: >= 38 participants
		// - Large size room: 16 ~ 17 participants
		// - Small size room: 5 ~ 6 participants
		// - One to one room: 2
		roomSize := conf.roomSize
		participantNumber := int(num)
		if participantNumber == 1 {
			return ParticipantError(errors.Wrapf(err, "there are no other participants in the conference room %q; meeting participant number got %d; want %d", room, num, roomSize))
		}
		switch roomSize {
		case ClassRoomSize:
			if participantNumber < roomSize {
				return ParticipantError(errors.Wrapf(err, "room url %q; meeting participant number got %d; want at least %d", room, num, roomSize))
			}
		case SmallRoomSize, LargeRoomSize:
			if participantNumber != roomSize && participantNumber != roomSize+1 {
				return ParticipantError(errors.Wrapf(err, "room url %q; meeting participant number got %d; want %d ~ %d", room, num, roomSize, roomSize+1))
			}
		case TwoRoomSize:
			if participantNumber != roomSize {
				return ParticipantError(errors.Wrapf(err, "room url %q; meeting participant number got %d; want %d", room, num, roomSize))
			}
		}

		return nil
	}

	targetMeetAccount := nodewith.Name(conf.account).Role(role.StaticText)
	return uiauto.Combine("join conference",
		openConference,
		allowPerm,
		// Check if the login account is the one for google meet. If not, switch to google meet account.
		ui.IfSuccessThen(ui.Gone(targetMeetAccount), switchUser),
		joinConf,
		// Sometimes participants number caught at the beginning is wrong, it will be correct after a while.
		// Add retry to get the correct participants number.
		ui.WithInterval(1*time.Second).Retry(5, checkParticipantsNum),
	)(ctx)
}

// VideoAudioControl controls the video and audio during conference.
func (conf *GoogleMeetConference) VideoAudioControl(ctx context.Context) error {
	// It may take some time to detect the microphone or camera button from the meet UI.
	const detectButtonTime = 30 * time.Second
	ui := uiauto.New(conf.tconn)

	toggleVideo := func(ctx context.Context) error {
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

	return uiauto.Combine("toggle video and audio",
		// Remain in the state for 5 seconds after each action.
		toggleVideo, ui.Sleep(5*time.Second),
		toggleVideo, ui.Sleep(5*time.Second),
		toggleAudio, ui.Sleep(5*time.Second),
		toggleAudio, ui.Sleep(5*time.Second),
	)(ctx)
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

	// Switch tab.
	if err := kb.Accel(ctx, "Ctrl+Tab"); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}

	return nil
}

// ChangeLayout changes the conference UI layout.
func (conf *GoogleMeetConference) ChangeLayout(ctx context.Context) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)

	// Close all notifications to prevent them from covering the print button.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close otifications")
	}
	moreOptions := nodewith.Name("More options").First()
	menu := nodewith.Name("Call options").Role(role.Menu)
	changeLayoutButton := nodewith.Name("Change layout").Role(role.MenuItem)
	changeLayoutPanel := nodewith.Name("Change layout").Role(role.Dialog)
	closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(changeLayoutPanel)
	for _, mode := range []string{"Tiled", "Spotlight"} {
		modeNode := nodewith.Name(mode).Role(role.RadioButton)
		changeLayout := func(ctx context.Context) error {
			testing.ContextLog(ctx, "Change layout to ", mode)
			return uiauto.Combine("change layout to "+mode,
				expandMenu(conf.tconn, moreOptions, menu, 433),
				ui.LeftClick(changeLayoutButton),
				ui.LeftClick(modeNode),
			)(ctx)
		}

		if err := uiauto.Combine("change layout",
			ui.Retry(3, changeLayout),
			ui.LeftClick(closeButton), // Close the layout panel.
			ui.Sleep(10*time.Second),  // After applying new layout, give it 10 seconds for viewing before applying next one.
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
		menu := nodewith.Name("Call options").Role(role.Menu)
		changeBackground := nodewith.Name("Change background").Role(role.MenuItem)
		backgroundButton := nodewith.Name(background).First()
		webArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
		closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(webArea)
		testing.ContextLog(ctx, "Change background to ", background)
		return uiauto.Combine("change background",
			expandMenu(conf.tconn, moreOptions, menu, 433),
			ui.LeftClick(changeBackground), // Open "Background" panel.
			ui.WithTimeout(30*time.Second).LeftClick(backgroundButton),
			ui.LeftClick(closeButton), // Close "Background" panel.
			ui.Sleep(5*time.Second),   // After applying new background, give it 5 seconds for viewing before applying next one.
		)(ctx)
	}
	pinBtn := nodewith.Name("Pin yourself to your main screen.")
	if err := ui.Exists(pinBtn)(ctx); err != nil {
		// If there are no participants in the room, the pin button will not be displayed.
		return ParticipantError(errors.Wrap(err, "failed to find the button to pin to main screen; other participants might have left"))
	}
	if err := ui.LeftClick(pinBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to pin to main screen")
	}

	for _, background := range []string{blurBackground, skyBackground, turnOffBackground} {
		if err := changeBackground(background); err != nil {
			return err
		}
	}

	return nil
}

// PresentSlide presents the slides to the conference.
func (conf *GoogleMeetConference) PresentSlide(ctx context.Context) error {
	const slideTitle = "Untitled presentation - Google Slides"
	tconn := conf.tconn
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	deletePerformed := false
	// slideCleanup switches to the slide page and deletes it.
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
	// Delete slide if any error occures.
	defer func() {
		if err := slideCleanup(cleanUpSlideCtx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
		}
	}()

	if err := uiauto.Combine("present slide",
		conf.switchToChromeTab("Meet"),
		conf.shareScreen(tconn, false),
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
		conf.switchToChromeTab("Meet"),
	)(ctx); err != nil {
		return err
	}

	return nil
}

// ExtendedDisplayPresenting presents the screen on dextended display.
func (conf *GoogleMeetConference) ExtendedDisplayPresenting(ctx context.Context) error {
	const slideTitle = "Untitled presentation - Google Slides"
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	deletePerformed := false
	// slideCleanup switches to the slide page and deletes it.
	slideCleanup := func(ctx context.Context) error {
		if deletePerformed {
			return nil
		}
		deletePerformed = true // Set it to true because we only try to delete once.
		webArea := nodewith.Name(slideTitle).Role(role.RootWebArea)
		testing.ContextLog(ctx, "Switch to the slide page and delete it")
		return uiauto.Combine("switch to the slide page and delete it",
			ui.IfSuccessThen(ui.Gone(webArea), kb.AccelAction("Alt+Tab")),
			deleteSlide(conf.tconn),
		)(ctx)
	}

	// Shorten the context to cleanup slide.
	cleanUpSlideCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Create a new Google Slides")
	if err := newGoogleSlides(ctx, conf.cr, true); err != nil {
		return err
	}
	// Delete slide if any error occures.
	defer func() {
		if err := slideCleanup(cleanUpSlideCtx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
		}
	}()

	moveConferenceTab := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Move conference tab to extended display")
		return uiauto.Combine("move conference to exteneded display",
			kb.AccelAction("Alt+Tab"),
			ui.Sleep(400*time.Millisecond),
			kb.AccelAction("Search+Alt+M"),
			ui.Sleep(400*time.Millisecond),
		)(ctx)
	}
	return uiauto.Combine("present slide",
		kb.AccelAction("Alt+Tab"), // Press Alt+Tab to switch to conference page.
		conf.shareScreen(tconn, true),
		moveConferenceTab,
		kb.AccelAction("Alt+Tab"), // Press Alt+Tab to switch to slide page.
		presentSlide(tconn, kb),
		editSlide(tconn, kb),
		func(ctx context.Context) error {
			if err := slideCleanup(ctx); err != nil {
				// Only log the error.
				testing.ContextLog(ctx, "Failed to clean up the slide: ", err)
			}
			return nil
		},
		kb.AccelAction("Alt+Tab"), // Press Alt+Tab to switch to conference page.
	)(ctx)
}

// StopPresenting stops the presentation mode.
func (conf *GoogleMeetConference) StopPresenting(ctx context.Context) error {
	ui := uiauto.New(conf.tconn)
	meetWebArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
	// There are two "Stop presenting" buttons on the screen with the same ancestor, role and name that we can't use unique finder.
	stopPresentingButton := nodewith.Name("Stop presenting").Role(role.Button).Ancestor(meetWebArea).First()
	testing.ContextLog(ctx, "Stop presenting")
	return ui.LeftClickUntil(stopPresentingButton, ui.WithTimeout(3*time.Second).WaitUntilGone(stopPresentingButton))(ctx)
}

// End ends the conference.
func (conf *GoogleMeetConference) End(ctx context.Context) error {
	return cuj.CloseAllWindows(ctx, conf.tconn)
}

var _ Conference = (*GoogleMeetConference)(nil)

// switchToChromeTab switch to the given chrome tab.
//
// TODO: Merge to cuj.UIActionHandler and introduce UIActionHandler in this test. See
// https://chromium-review.googlesource.com/c/chromiumos/platform/tast-tests/+/2779315/
func (conf *GoogleMeetConference) switchToChromeTab(tabName string) action.Action {
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

// shareScreen share screen from google meet.
func (conf *GoogleMeetConference) shareScreen(tconn *chrome.TestConn, extendedDisplay bool) action.Action {
	const slideTitle = "Untitled presentation - Google Slides"
	ui := uiauto.New(tconn)
	meetWebArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
	menu := nodewith.Name("Presentation options").Role(role.Menu).Ancestor(meetWebArea)
	presentNowButton := nodewith.Name("Present now").Ancestor(meetWebArea)
	presentMode := nodewith.Name("A window").Role(role.MenuItem)
	presentWindow := nodewith.ClassName("DesktopMediaSourceView").First()
	shareButton := nodewith.Name("Share").Role(role.Button)
	// There are two "Stop presenting" buttons on the screen with the same ancestor, role and name that we can't use unique finder.
	stopPresenting := nodewith.Name("Stop presenting").Role(role.Button).Ancestor(meetWebArea).First()
	if extendedDisplay {
		presentMode = nodewith.NameContaining("A tab").Role(role.MenuItem)
		presentWindow = nodewith.ClassName("AXVirtualView").Role(role.Cell).Name(slideTitle)
	}

	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start to share screen")
		return uiauto.Combine("share screen",
			expandMenu(conf.tconn, presentNowButton, menu, 172),
			ui.LeftClick(presentMode),
			ui.LeftClick(presentWindow),
			ui.LeftClickUntil(shareButton, ui.Gone(shareButton)),
			ui.WithTimeout(time.Minute).WaitUntilExists(stopPresenting),
		)(ctx)
	}
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

// expandMenu returns a function that clicks the button and waits for the menu to expand to the given height.
// This function is useful when the target menu will expand to its full size with animation. On Low end DUTs
// the expansion animation might stuck for some time. The node might have returned a stable location if
// checking with a fixed interval before the animiation completes. This function ensures animation completes
// by checking the menu height.
func expandMenu(tconn *chrome.TestConn, button, menu *nodewith.Finder, height int) action.Action {
	ui := uiauto.New(tconn)
	startTime := time.Now()
	return func(ctx context.Context) error {
		if err := ui.LeftClick(button)(ctx); err != nil {
			return errors.Wrap(err, "failed to click button")
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			menuInfo, err := ui.Info(ctx, menu)
			if err != nil {
				return errors.Wrap(err, "failed to get menu info")
			}
			if menuInfo.Location.Height < height {
				return errors.Errorf("got menu height %d, want %d", menuInfo.Location.Height, height)
			}
			// Examine this log regularly to see how fast the menu is expanded and determine if
			// we still need to keep this expandMenu() function.
			testing.ContextLog(ctx, "Menu expanded to full height in ", time.Now().Sub(startTime))
			return nil
		}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second})
	}
}
