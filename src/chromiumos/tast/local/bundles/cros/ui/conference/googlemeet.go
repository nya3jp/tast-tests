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
	cr       *chrome.Chrome
	tconn    *chrome.TestConn
	roomSize int
	account  string
	password string
}

// Join joins a new conference room.
func (conf *GoogleMeetConference) Join(ctx context.Context, room string) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	meetAccount := conf.account

	// The Chrome browser would ask for camera and microphone permissions
	// before joining Google Meet conference. allowPerm allows both of camera
	// and audio permissions.
	allowPerm := func(ctx context.Context) error {
		dismissButton := nodewith.Name("Dismiss").Role(role.Button)
		allowButton := nodewith.Name("Allow").Role(role.Button)
		notification := nodewith.Name("Show notifications").First()
		if err := ui.WaitUntilExists(dismissButton)(ctx); err == nil {
			if err := ui.LeftClickUntil(allowButton, ui.Gone(dismissButton))(ctx); err != nil {
				return errors.Wrap(err, "failed to allow permissions")
			}
			if err := ui.WaitUntilExists(notification)(ctx); err == nil {
				if err := ui.LeftClick(allowButton)(ctx); err != nil {
					return errors.Wrap(err, "failed to allow permissions")
				}
			}
		}
		return nil
	}

	joinConf := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Join Conference")
		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		if err := uiauto.Combine("Join conference",
			ui.WaitUntilExists(joinNowButton),
			ui.LeftClickUntil(joinNowButton, ui.Gone(nodewith.Name("Return to home screen"))),
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

		if err := ui.WaitUntilExists(useAnotherAccount)(ctx); err != nil {
			return errors.Wrap(err, "failed to use another account")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ui.LeftClick(useAnotherAccount)(ctx)
			// Shown in the first time
			if err := ui.LeftClick(viewAccounts)(ctx); err != nil {
				testing.ContextLog(ctx, "View accounts is not shown")
			}
			if err := ui.WithTimeout(3 * time.Second).Exists(myAccounts)(ctx); err != nil {
				return errors.Wrap(err, `failed to wait "My accounts"`)
			}
			return nil
		}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
			return err
		}

		addAccountButton := nodewith.Name("Add account").Role(role.Button)
		emailField := nodewith.Name("Email or phone").Role(role.TextField)
		nextButton := nodewith.Name("Next").Role(role.Button)
		passwordField := nodewith.Name("Enter your password").Role(role.TextField)
		iAgree := nodewith.Name("I agree").Role(role.Button)
		if err := uiauto.Combine("Add account",
			ui.WaitUntilExists(addAccountButton),
			ui.LeftClick(addAccountButton),
			ui.WaitUntilExists(emailField),
			ui.LeftClick(emailField),
			kb.TypeAction(meetAccount),
			ui.WaitUntilExists(nextButton),
			ui.LeftClick(nextButton),
			ui.WaitUntilExists(passwordField),
			ui.WithInterval(2*time.Second).LeftClick(passwordField),
			kb.TypeAction(conf.password),
			ui.WaitUntilExists(nextButton),
			ui.LeftClick(nextButton),
			ui.WaitUntilExists(iAgree),
			ui.LeftClick(iAgree),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to add account")
		}

		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			return errors.Wrap(err, `failed to close settings page`)
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
		testing.ContextLog(ctx, "Switch account")
		switchAccount := nodewith.Name("Switch account").Role(role.Link)
		meetAccountText := nodewith.Name(meetAccount).First()
		chooseAnAccount := nodewith.Name("Choose an account").First()
		if err := uiauto.Combine("Switch account",
			ui.WaitUntilExists(switchAccount),
			ui.LeftClickUntil(switchAccount, ui.Gone(switchAccount)),
			ui.WaitUntilExists(chooseAnAccount),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch account")
		}

		if err := ui.WaitUntilExists(meetAccountText)(ctx); err != nil {
			if err := addMeetAccount(ctx); err != nil {
				return errors.Wrapf(err, "failed to add account %s", meetAccount)
			}
		}
		testing.ContextLogf(ctx, "Select meet account: %s", meetAccount)

		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		if err := uiauto.Combine("Switch account",
			ui.WaitUntilExists(meetAccountText),
			ui.WithTimeout(time.Minute).LeftClickUntil(meetAccountText, ui.Exists(joinNowButton)),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to switch account to %s", meetAccount)
		}
		return nil
	}

	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size
	checkParticipantsNum := func(ctx context.Context) error {
		showEveryone := nodewith.Name("Show everyone").Role(role.Button)
		people := nodewith.Name("People").First()
		// Some of DUT models with poor performace. When the case join
		// a large conference (over 15 participants), it would take much time
		// to render DOM elements.
		if err := uiauto.Combine("Click 'Show everyone'",
			ui.WithTimeout(2*time.Minute).WaitUntilExists(showEveryone),
			ui.LeftClick(showEveryone),
			ui.WaitUntilExists(people),
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
		if int(num) < conf.roomSize {
			return errors.Wrap(err, `the number of participants is not as expected`)
		}
		if int(num) > conf.roomSize {
			testing.ContextLogf(ctx, "There are %d participants, more than expectation", num)
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
	tconn := conf.tconn
	toggleVideo := func(ctx context.Context) error {
		const (
			stopVideo  = "Turn off camera (ctrl + e)"
			startVideo = "Turn on camera (ctrl + e)"
		)
		ui := uiauto.New(tconn)
		stopVideoButton := nodewith.Name(stopVideo).Role(role.Button)
		startVideoButton := nodewith.Name(startVideo).Role(role.Button)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := ui.LeftClick(stopVideoButton)(ctx); err != nil {
				if err := ui.LeftClick(startVideoButton)(ctx); err != nil {
					return err
				}
				testing.ContextLog(ctx, "Open Video")
				return nil
			}
			testing.ContextLog(ctx, "Close Video")
			return nil
		}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
			return err
		}
		return nil
	}

	toggleAudio := func(ctx context.Context) error {
		const (
			muteAudio   = "Turn off microphone (ctrl + d)"
			unmuteAudio = "Turn on microphone (ctrl + d)"
		)
		ui := uiauto.New(tconn)
		muteAudioButton := nodewith.Name(muteAudio).Role(role.Button)
		unmuteAudioButton := nodewith.Name(unmuteAudio).Role(role.Button)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := ui.LeftClick(muteAudioButton)(ctx); err != nil {
				if err := ui.LeftClick(unmuteAudioButton)(ctx); err != nil {
					return err
				}
				testing.ContextLog(ctx, "Open Audio")
				return nil
			}
			testing.ContextLog(ctx, "Mute Audio")
			return nil
		}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
			return err
		}
		return nil
	}

	testing.ContextLog(ctx, "Turn off camera")
	if err := toggleVideo(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	testing.ContextLog(ctx, "Turn on camera")
	if err := toggleVideo(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle video")
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	testing.ContextLog(ctx, "Toggle Audio")
	if err := toggleAudio(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle audio")
	}

	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
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
	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close notifications")
	}

	moreOptions := nodewith.Name("More options").First()
	changeLayout := nodewith.Name("Change layout").Role(role.MenuItem)
	for _, mode := range []string{"Tiled", "Spotlight"} {
		modeNode := nodewith.Name(mode).Role(role.RadioButton)
		if err := uiauto.Combine("ChangeLayout",
			ui.WaitUntilExists(moreOptions),
			ui.LeftClick(moreOptions),
			ui.WaitUntilExists(changeLayout),
			ui.LeftClick(changeLayout),
			ui.WaitUntilExists(modeNode),
			ui.LeftClick(modeNode),
			ui.LeftClick(moreOptions),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to change layout")
		}
		// After applying new layout, give it 10 seconds for viewing
		// before applying next one.
		if err := testing.Sleep(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait layout change")
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
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	changeBackground := func(background string) error {
		moreOptions := nodewith.Name("More options").First()
		changeBackground := nodewith.Name("Change background").Role(role.MenuItem)
		backgroundButton := nodewith.Name(background).First()
		closeButton := nodewith.Name("Backgrounds").Role(role.Button)
		if err := uiauto.Combine("Change background",
			ui.WaitUntilExists(moreOptions),
			ui.LeftClick(moreOptions),
			ui.WaitUntilExists(changeBackground),
			ui.LeftClick(changeBackground),
			ui.WithTimeout(30*time.Second).WaitUntilExists(backgroundButton),
			ui.LeftClick(backgroundButton),
			ui.LeftClick(closeButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to change background")
		}
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		return nil
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
		if err := uiauto.Combine("Present Slide",
			ui.WaitUntilExists(presentNowButton),
			ui.LeftClick(presentNowButton),
			ui.WaitUntilExists(aWindow),
			ui.LeftClick(aWindow),
			ui.WaitUntilExists(presentWindow),
			ui.LeftClick(presentWindow),
			ui.WaitUntilExists(shareButton),
			ui.LeftClickUntil(shareButton, ui.Gone(shareButton)),
			ui.WithTimeout(2*time.Minute).WaitUntilExists(stopPresenting),
		)(ctx); err != nil {
			return err
		}

		return nil
	}

	switchTab := func(ctx context.Context, tabName string) error {
		tab := nodewith.NameRegex(regexp.MustCompile(tabName)).Role(role.Tab)
		if err := uiauto.Combine(`Click tab`,
			ui.WaitUntilExists(tab),
			ui.LeftClick(tab),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click tab")
		}
		return nil
	}

	testing.ContextLog(ctx, "Switch to conference page")
	if err := switchTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}

	testing.ContextLog(ctx, "Click present now button")
	if err := shareScreen(ctx, tconn); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switch to the slide")
	if err := switchTab(ctx, slideTitle); err != nil {
		return errors.Wrap(err, "failed to switch tab to slide page")
	}

	testing.ContextLog(ctx, "Start present slide")
	if err := presentSlide(ctx, tconn, kb); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Edit slide")
	if err := editSlide(ctx, tconn, kb); err != nil {
		return errors.Wrap(err, `failed to edit slide when leave presentation mode`)
	}

	testing.ContextLog(ctx, "Switch to conference page")
	if err := switchTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}

	return nil
}

// ExtendedDisplayPresenting presents the screen on dextended display.
func (conf *GoogleMeetConference) ExtendedDisplayPresenting(ctx context.Context, tabletMode bool) error {
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
		settings, err := ossettings.LaunchAtPage(ctx, tconn, ossettings.Device)
		if err != nil {
			return errors.Wrap(err, "failed to launch os-settings page")
		}

		if err := settings.LaunchDisplay()(ctx); err != nil {
			return errors.Wrap(err, "failed to launch display page")
		}

		if err := settings.ClickMirrorDisplay()(ctx); err != nil {
			return errors.Wrap(err, "failed to click mirror display")
		}

		if err := settings.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close settings")
		}

		return nil
	}

	moveConferenceTab := func(ctx context.Context) error {
		if err := kb.Accel(ctx, "Alt+Tab"); err != nil {
			return errors.Wrap(err, "failed to switch to conference page")
		}
		testing.Sleep(ctx, 400*time.Millisecond)
		if err := kb.Accel(ctx, "Search+Alt+M"); err != nil {
			return errors.Wrap(err, "failed to move tab to extended display")
		}
		testing.Sleep(ctx, 400*time.Millisecond)
		return nil
	}

	enterPresentingMode := func(ctx context.Context) error {
		presentNowButton := nodewith.Name("Present now").First()
		aWindow := nodewith.Name("A window").First()
		if err := uiauto.Combine("Share Screen",
			kb.AccelAction("Alt+Tab"),
			ui.WaitUntilExists(presentNowButton),
			ui.LeftClick(presentNowButton),
			ui.WaitUntilExists(aWindow),
			ui.LeftClick(aWindow),
		)(ctx); err != nil {
			return err
		}
		return nil
	}

	selectSharedWindow := func(ctx context.Context) error {
		presentWindow := nodewith.ClassName("DesktopMediaSourceView").NameRegex(regexp.MustCompile("My Drive"))
		shareButton := nodewith.Name("Share").Role(role.Button)
		stopPresentation := nodewith.Name("Stop presenting").Role(role.Button)
		if err := uiauto.Combine("Select Shared Tab",
			ui.WaitUntilExists(presentWindow),
			ui.LeftClick(presentWindow),
			ui.WaitUntilExists(shareButton),
			ui.LeftClickUntil(shareButton, ui.Gone(shareButton)),
			ui.WaitUntilGone(stopPresentation),
		)(ctx); err != nil {
			return err
		}
		return nil
	}
	switchTab := func(ctx context.Context, tabName string) error {
		tab := nodewith.NameRegex(regexp.MustCompile(tabName)).Role(role.Tab)
		if err := uiauto.Combine(`Click tab`,
			ui.WaitUntilExists(tab),
			ui.LeftClick(tab),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click tab")
		}
		return nil
	}

	if tabletMode {
		if err := unsetMirrorDisplay(ctx, tconn); err != nil {
			return err
		}
	}

	testing.ContextLog(ctx, "Switch to conference page")
	if err := switchTab(ctx, "Meet"); err != nil {
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
	if err := switchTab(ctx, slideTitle); err != nil {
		return errors.Wrap(err, "failed to switch tab to slide page")
	}

	testing.ContextLog(ctx, "Start present slide")
	if err := presentSlide(ctx, tconn, kb); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Edit slide")
	if err := editSlide(ctx, tconn, kb); err != nil {
		return errors.Wrap(err, `failed to edit slide when leave presentation mode`)
	}

	testing.ContextLog(ctx, "Switch to conference page")
	if err := switchTab(ctx, "Meet"); err != nil {
		return errors.Wrap(err, "failed to switch tab to conference page")
	}
	return nil
}

// StopPresenting stops the presentation mode.
func (conf *GoogleMeetConference) StopPresenting(ctx context.Context) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	stopPresentingButton := nodewith.Name("Stop presenting").Role(role.Button)
	testing.ContextLog(ctx, "Stop presenting")
	if err := ui.LeftClickUntil(stopPresentingButton, ui.Gone(stopPresentingButton))(ctx); err != nil {
		return err
	}

	return nil
}

// End ends the conference.
func (conf *GoogleMeetConference) End(ctx context.Context) error {
	cuj.CloseAllWindows(ctx, conf.tconn)
	return nil
}

var _ Conference = (*GoogleMeetConference)(nil)

// NewGoogleMeetConference creates Google Meet conference room instance which implements Conference interface.
func NewGoogleMeetConference(cr *chrome.Chrome, tconn *chrome.TestConn, roomSize int, account, password string) *GoogleMeetConference {
	return &GoogleMeetConference{cr, tconn, roomSize, account, password}
}
