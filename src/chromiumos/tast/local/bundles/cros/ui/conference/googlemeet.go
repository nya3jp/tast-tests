// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// GoogleMeetConference implements the Conference interface.
type GoogleMeetConference struct {
	cr                         *chrome.Chrome
	br                         *browser.Browser
	tconn                      *chrome.TestConn
	kb                         *input.KeyboardEventWriter
	ui                         *uiauto.Context
	uiHandler                  cuj.UIActionHandler
	displayAllParticipantsTime time.Duration
	tabletMode                 bool
	extendedDisplay            bool
	bt                         browser.Type
	roomSize                   int
	networkLostCount           int
	account                    string
	password                   string
	outDir                     string
	room                       string
}

const (
	turnOffBackground = "Turn off visual effects"
	blurBackground    = "Blur your background"
	staticBackground  = "Blurry sky with purple horizon"
	dynamicBackground = "Spaceship"
	retryTimes        = 3
)

// Join joins a new conference room.
func (conf *GoogleMeetConference) Join(ctx context.Context, room string, toBlur bool) error {
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	kb := conf.kb
	meetAccount := conf.account
	conf.room = room
	openConference := func(ctx context.Context) error {
		// Set newWindow to true to launch zoom in the first Chrome tab.
		conn, err := conf.uiHandler.NewChromeTab(ctx, conf.br, room, true)
		if err != nil {
			return CheckSignedOutError(ctx, tconn, errors.Wrap(err, "failed to create chrome connection to join the conference"))
		}
		if err := webutil.WaitForQuiescence(ctx, conn, longUITimeout); err != nil {
			return CheckSignedOutError(ctx, tconn, errors.Wrapf(err, "failed to wait for %q to be loaded and achieve quiescence", room))
		}
		// Maximize the meet window to show all the browser UI elements for precise clicking.
		if !conf.tabletMode {
			// Find the meet browser window.
			window, err := ash.FindWindow(ctx, conf.tconn, func(w *ash.Window) bool {
				return (w.WindowType == ash.WindowTypeBrowser || w.WindowType == ash.WindowTypeLacros) && strings.Contains(w.Title, "Meet")
			})
			if err != nil {
				return errors.Wrap(err, "failed to find the meet window")
			}
			if err := ash.SetWindowStateAndWait(ctx, conf.tconn, window.ID, ash.WindowStateMaximized); err != nil {
				// Just log the error and try to continue.
				testing.ContextLog(ctx, "Try to continue the test even though maximizing the meet window failed: ", err)
			}
		}
		return nil
	}
	//  allowPerm allows camera, microphone and notification if browser asks for the permissions.
	allowPerm := func(ctx context.Context) error {
		video := nodewith.Role(role.Video)
		allowButton := nodewith.Name("Allow").Role(role.Button)
		dismissButton := nodewith.Name("Dismiss").Role(role.Button)
		avPerm := nodewith.NameRegex(regexp.MustCompile(".*Use your (microphone|camera).*")).ClassName("RootView").Role(role.AlertDialog).First()
		notiPerm := nodewith.NameContaining("Show notifications").ClassName("RootView").Role(role.AlertDialog)
		// If there is a video, it means permissions are allowed.
		if err := ui.WithTimeout(shortUITimeout).WaitUntilExists(video)(ctx); err == nil {
			return nil
		}
		for _, step := range []struct {
			name   string
			finder *nodewith.Finder
			button *nodewith.Finder
		}{
			{"dismiss permission prompt", dismissButton, dismissButton},
			// Some DUTs show allow notifications first. Some don't.
			{"allow notifications", notiPerm, allowButton.Ancestor(notiPerm)},
			{"allow microphone and camera", avPerm, allowButton.Ancestor(avPerm)},
			{"allow notifications", notiPerm, allowButton.Ancestor(notiPerm)},
		} {
			if err := ui.WithTimeout(shortUITimeout).WaitUntilExists(step.finder)(ctx); err == nil {
				// Immediately clicking the allow button sometimes doesn't work. Sleep 2 seconds.
				if err := uiauto.NamedAction(step.name,
					ui.DoDefaultUntil(step.button, ui.WithTimeout(shortUITimeout).WaitUntilGone(step.finder)))(ctx); err != nil {
					return err
				}
			} else {
				testing.ContextLog(ctx, "No action is required to ", step.name)
			}
		}
		return allowPagePermissions(tconn)(ctx)
	}
	switchWindow := func(ctx context.Context) error {
		// Default expected display is main display.
		if err := cuj.SwitchWindowToDisplay(ctx, tconn, kb, conf.extendedDisplay)(ctx); err != nil {
			if conf.extendedDisplay {
				return errors.Wrap(err, "failed to switch conference window to the extended display")
			}
			return errors.Wrap(err, "failed to switch conference window to the internal display")
		}
		return nil
	}
	changeBackgroundToBlur := func(ctx context.Context) error {
		if !toBlur {
			return nil
		}
		changeBackgroundButton := nodewith.Name("Apply visual effects").Role(role.Button)
		noEffectAndBlurRegion := nodewith.NameContaining("No effect & blur").Role(role.Region)
		noEffectAndBlurHeading := nodewith.NameContaining("No effect & blur").Role(role.Heading)
		turnOffButton := nodewith.NameContaining(turnOffBackground).Role(role.ToggleButton)
		backgroundButton := nodewith.Name(blurBackground).Role(role.ToggleButton)
		webArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
		selectAFileDialog := nodewith.Name("Select a file to open").ClassName("ExtensionViewViews")
		closeDialog := nodewith.Name("Close").Role(role.Button).Ancestor(selectAFileDialog)
		closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(webArea)
		return uiauto.NamedAction("change background to blur",
			uiauto.Combine("change background to blur",
				ui.LeftClick(changeBackgroundButton), // Open "Background" panel.
				ui.WithTimeout(longUITimeout).WaitUntilExists(noEffectAndBlurRegion),
				ui.LeftClick(noEffectAndBlurHeading),
				// Turn off effect to avoid clicking the blur button to turn off the effect.
				cuj.ExpandMenu(conf.tconn, turnOffButton, noEffectAndBlurRegion, 100),
				ui.LeftClick(backgroundButton),
				ui.WaitUntilExists(backgroundButton.Focused()),
				takeScreenshot(conf.cr, conf.outDir, "change-background-to-blur"),
				ui.LeftClick(closeButton), // Close "Background" panel.
				// Some DUT performance is too poor, clicking the turn off button will trigger "Upload a background image".
				// If the dialog "select a file to open" is opened, close it.
				uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(selectAFileDialog), ui.LeftClick(closeDialog)),
			))(ctx)
	}
	joinConf := func(ctx context.Context) error {
		actionName := "join conference"
		joinButton := nodewith.NameRegex(regexp.MustCompile("(Join now|Ask to join)")).Role(role.Button)
		joinNowButton := nodewith.Name("Join now").Role(role.Button)
		homeScreenLink := nodewith.Name("Return to home screen").Role(role.Link)
		// Some low-end devices need more time to wait "Join now" button animation loading.
		waitJoinNowButtonStable := func(ctx context.Context) error {
			joinForAudioButton := nodewith.Name("Join and use a phone for audio").Role(role.Button)
			return testing.Poll(ctx, func(ctx context.Context) error {
				joinForAudioLocation, err := ui.Location(ctx, joinForAudioButton)
				if err != nil {
					// No need to compare if joinForAudioButton doesn't exist. Return nil here.
					return nil
				}
				joinNowLocation, err := ui.Location(ctx, joinNowButton)
				if err != nil {
					return errors.Wrap(err, "failed to get location of the join now button")
				}
				joinNowButtonHeight := joinNowLocation.CenterY()
				joinForAudioButtonHeight := joinForAudioLocation.CenterY()
				testing.ContextLogf(ctx, "Check that the join now button (%d) should be lower than the join for audio button (%d)", joinNowButtonHeight, joinForAudioButtonHeight)
				if joinNowButtonHeight >= joinForAudioButtonHeight {
					return errors.Wrap(err, "the 'Join now' button is not on the correct location")
				}
				return nil
			}, &testing.PollOptions{Timeout: shortUITimeout})
		}
		startTime := time.Now()
		if err := ui.WithTimeout(longUITimeout).WaitUntilExists(joinButton)(ctx); err != nil {
			return errors.Wrapf(err, "failed to wait for the join button within %v", longUITimeout)
		}
		testing.ContextLogf(ctx, "The join button took %v to appear", time.Now().Sub(startTime))
		return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
			waitJoinNowButtonStable,
			ui.RetryUntil(ui.DoDefault(joinButton), ui.WithTimeout(shortUITimeout).WaitUntilGone(joinButton)),
			ui.WithTimeout(longUITimeout).WaitUntilGone(homeScreenLink),
		))(ctx)
	}

	// enterAccount enter account email and password.
	enterAccount := func(ctx context.Context) error {
		emailContent := nodewith.NameContaining(meetAccount).Role(role.InlineTextBox).Editable()
		emailField := nodewith.Name("Email or phone").Role(role.TextField)
		nextButton := nodewith.Name("Next").Role(role.Button)
		passwordField := nodewith.Name("Enter your password").Role(role.TextField)
		iAgree := nodewith.Name("I agree").Role(role.Button)
		var actions []uiauto.Action
		// If emailContent is not found, it should fill in the account.
		if err := ui.WithTimeout(shortUITimeout).WaitUntilExists(emailContent)(ctx); err != nil {
			// Email has not been entered into the text box yet.
			actions = append(actions,
				// Make sure text area is focused before typing. This is especially necessary on low-end DUTs.
				uiauto.NamedAction("click email field",
					ui.WithTimeout(longUITimeout).LeftClickUntil(emailField,
						ui.WithTimeout(shortUITimeout).WaitUntilExists(emailField.Focused()))),
				uiauto.NamedAction("type account", kb.TypeAction(meetAccount)),
			)
		}
		actions = append(actions,
			// The "Sign-in again" notification will block the next button, close it.
			func(ctx context.Context) error {
				return ash.CloseNotifications(ctx, tconn)
			},
			ui.LeftClick(nextButton),
			// Make sure text area is focused before typing. This is especially necessary on low-end DUTs.
			ui.LeftClickUntil(passwordField, ui.Exists(passwordField.Focused())),
			kb.TypeAction(conf.password),
			ui.LeftClick(nextButton),
			ui.LeftClickUntil(iAgree, ui.WithTimeout(shortUITimeout).WaitUntilGone(iAgree)),
		)

		testing.ContextLog(ctx, "Enter email and password")
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
			// The ui of Chrome and Lacros are different when adding account.
			if conf.bt == browser.TypeLacros {
				continueButton := nodewith.Name("Continue").Role(role.Button)
				if err := ui.LeftClick(continueButton)(ctx); err != nil {
					return err
				}
			} else {
				dontReminder := nodewith.Name("Don't remind me next time").Role(role.CheckBox)
				signInWebArea := nodewith.Name("Sign in to add a Google account").Role(role.RootWebArea)
				okBtn := nodewith.Name("OK").Role(role.Button).Ancestor(signInWebArea)
				if err := uiauto.Combine("close dialog",
					ui.LeftClick(dontReminder),
					ui.LeftClick(okBtn))(ctx); err != nil {
					return err
				}
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
			ui.DoDefaultUntil(switchAccount, ui.WithTimeout(shortUITimeout).WaitUntilGone(switchAccount)),
			ui.WaitUntilExists(chooseAnAccount),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch account")
		}
		// If meet account doesn't exist, add the account first.
		if err := ui.WithTimeout(shortUITimeout).WaitUntilExists(meetAccountText)(ctx); err != nil {
			testing.ContextLogf(ctx, "Add additional account %s to existing account", meetAccount)
			if err := addMeetAccount(ctx); err != nil {
				return errors.Wrapf(err, "failed to add account %s", meetAccount)
			}
		}
		testing.ContextLog(ctx, "Select meet account ", meetAccount)
		nextUI := nodewith.NameRegex(regexp.MustCompile("(Join now|Ask to join|Email or phone)")).First()
		if err := uiauto.Combine("select account",
			ui.WaitUntilExists(meetAccountText),
			ui.WithTimeout(longUITimeout).LeftClickUntil(meetAccountText, ui.WaitUntilExists(nextUI)),
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
		return nil
	}
	meetWebArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size.
	checkParticipantsNum := func(ctx context.Context) error {
		participant := nodewith.NameRegex(regexp.MustCompile(`^[\d]+$`)).Role(role.StaticText).Ancestor(meetWebArea)
		if err := uiauto.Combine("wait for the meet page to load participant",
			conf.closeNotifDialog(),
			// Some DUT models have poor performance. When joining
			// a large conference (over 15 participants), it would take much time
			// to render DOM elements. Set a longer timer here.
			ui.WithTimeout(longUITimeout).WaitUntilExists(participant),
		)(ctx); err != nil {
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
		switchWindow,
		// Check if the login account is the one for google meet. If not, switch to google meet account.
		uiauto.IfSuccessThen(ui.Gone(targetMeetAccount), switchUser),
		changeBackgroundToBlur,
		joinConf,
		ui.WithTimeout(longUITimeout).WaitUntilExists(meetWebArea),
		// Sometimes participants number caught at the beginning is wrong, it will be correct after a while.
		// Add retry to get the correct participants number.
		ui.WithInterval(time.Second).Retry(5, checkParticipantsNum),
	)(ctx)
}

// VideoAudioControl controls the video and audio during conference.
func (conf *GoogleMeetConference) VideoAudioControl(ctx context.Context) error {
	ui := conf.ui
	toggleVideo := func(ctx context.Context) error {
		cameraButton := nodewith.NameRegex(regexp.MustCompile("Turn (on|off) camera.*")).Role(role.Button)
		info, err := ui.WithTimeout(mediumUITimeout).Info(ctx, cameraButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet camera switch button to show")
		}
		if strings.HasPrefix(info.Name, "Turn on") {
			testing.ContextLog(ctx, "Turn camera from off to on")
		} else {
			testing.ContextLog(ctx, "Turn camera from on to off")
		}
		cameraButton = nodewith.Name(info.Name).Role(role.Button)
		if err := ui.WithTimeout(mediumUITimeout).DoDefaultUntil(cameraButton, ui.WaitUntilGone(cameraButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to switch camera")
		}
		return nil
	}

	toggleAudio := func(ctx context.Context) error {
		microphoneButton := nodewith.NameRegex(regexp.MustCompile("Turn (on|off) microphone.*")).Role(role.Button)
		info, err := ui.WithTimeout(mediumUITimeout).Info(ctx, microphoneButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet microphone switch button to show")
		}
		if strings.HasPrefix(info.Name, "Turn on") {
			testing.ContextLog(ctx, "Turn microphone from off to on")
		} else {
			testing.ContextLog(ctx, "Turn microphone from on to off")
		}
		microphoneButton = nodewith.Name(info.Name).Role(role.Button)
		if err := ui.WithTimeout(mediumUITimeout).DoDefaultUntil(microphoneButton, ui.WaitUntilGone(microphoneButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to switch microphone")
		}
		return nil
	}
	return uiauto.Combine("toggle video and audio",
		conf.closeNotifDialog(),
		// Remain in the state for 5 seconds after each action.
		toggleVideo, uiauto.Sleep(viewingTime),
		toggleVideo, uiauto.Sleep(viewingTime),
		toggleAudio, uiauto.Sleep(viewingTime),
		toggleAudio, uiauto.Sleep(viewingTime),
	)(ctx)
}

// SwitchTabs switches the chrome tabs.
func (conf *GoogleMeetConference) SwitchTabs(ctx context.Context) error {
	testing.ContextLog(ctx, "Open wiki page")
	// Set newWindow to false to make the tab in the same Chrome window.
	wikiConn, err := conf.uiHandler.NewChromeTab(ctx, conf.br, cuj.WikipediaURL, false)
	if err != nil {
		return errors.Wrap(err, "failed to open the wiki url")
	}
	defer wikiConn.Close()
	defer wikiConn.CloseTarget(ctx)
	if err := webutil.WaitForQuiescence(ctx, wikiConn, longUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait for wiki page to finish loading")
	}
	return uiauto.Combine("switch tab",
		uiauto.NamedAction("stay wiki page for 3 seconds", uiauto.Sleep(3*time.Second)),
		uiauto.NamedAction("switch to meet tab", conf.uiHandler.SwitchToChromeTabByName("Meet")),
		conf.checkLostNetwork,
	)(ctx)
}

// TypingInChat opens chat window and type.
func (conf *GoogleMeetConference) TypingInChat(ctx context.Context) error {
	const (
		actionName = "open chat window and type"
		message    = "Hello! How are you?"
	)
	chatButton := nodewith.Name("Chat with everyone").Role(role.ToggleButton)
	chatTextField := nodewith.Name("Send a message to everyone").Role(role.TextField)
	messageText := nodewith.NameContaining(message).Role(role.StaticText).First()
	return uiauto.NamedAction(actionName, conf.ui.Retry(retryTimes, uiauto.Combine(actionName,
		conf.ui.DoDefault(chatButton),
		// Some low end DUTs need very long time to load chat window in 49 tiles.
		conf.ui.WithTimeout(2*time.Minute).WaitUntilExists(chatTextField),
		conf.ui.WithTimeout(2*time.Minute).LeftClickUntil(chatTextField, conf.ui.WaitUntilExists(chatTextField.Focused())),
		conf.kb.TypeAction(message),
		conf.kb.AccelAction("enter"),
		conf.ui.WithTimeout(longUITimeout).WaitUntilExists(messageText),
		uiauto.Sleep(viewingTime), // After typing, wait 5 seconds for viewing.
		conf.ui.DoDefault(chatButton),
		conf.ui.WithTimeout(longUITimeout).WaitUntilGone(chatTextField),
	)))(ctx)
}

// SetLayoutMax sets the conference UI layout to max tiled grid.
func (conf *GoogleMeetConference) SetLayoutMax(ctx context.Context) error {
	return uiauto.Combine("set layout to max",
		conf.changeLayout("Tiled"),
		uiauto.Sleep(viewingTime), // After applying new layout, give it 5 seconds for viewing before applying next one.
	)(ctx)
}

// SetLayoutMin sets the conference UI layout to minimal tiled grid.
func (conf *GoogleMeetConference) SetLayoutMin(ctx context.Context) error {
	return uiauto.Combine("set layout to minimal",
		conf.changeLayout("Spotlight"),
		uiauto.Sleep(viewingTime), // After applying new layout, give it 5 seconds for viewing before applying next one.
	)(ctx)
}

// getGrids returns the current tiled grids.
func (conf *GoogleMeetConference) getGrids(ctx context.Context) (grids []uiauto.NodeInfo, err error) {
	webArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
	grid := nodewith.HasClass("xsj2Ff").Role(role.GenericContainer).Ancestor(webArea)
	grids, err = conf.ui.NodesInfo(ctx, grid)
	if err != nil {
		return grids, errors.Wrap(err, "failed to find grids")
	}
	return grids, nil
}

// getStableGrids returns stable tiled grids that take time to load.
// It calculates the displayAllParticipantsTime when the grid number doesn't change in a 5-second interval.
// The grids are not necessarily playing videos.
func (conf *GoogleMeetConference) getStableGrids(ctx context.Context) (grids []uiauto.NodeInfo, err error) {
	var loadingTime time.Duration
	lastQuantity := -1
	count := 0
	testing.ContextLog(ctx, "Wait for grids loading to stabilize")
	startTime := time.Now()
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		grids, err = conf.getGrids(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to find grids")
		}
		currentQuantity := len(grids)
		if currentQuantity == lastQuantity {
			count++
		} else {
			lastQuantity = currentQuantity
			loadingTime = time.Now().Sub(startTime)
			count = 0
		}
		if count > 5 {
			testing.ContextLogf(ctx, "There are currently %v grids displayed in %v", currentQuantity, loadingTime)
			return nil
		}
		return errors.New("Grids are still loading now")
	}, &testing.PollOptions{Interval: time.Second, Timeout: longUITimeout}); err != nil {
		return grids, err
	}
	conf.displayAllParticipantsTime = loadingTime
	return grids, nil
}

// changeLayout changes the conference UI layout.
func (conf *GoogleMeetConference) changeLayout(mode string) action.Action {
	return func(ctx context.Context) error {
		tconn := conf.tconn
		ui := conf.ui
		// Close all notifications to prevent them from covering the print button.
		if err := ash.CloseNotifications(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close notifications")
		}
		moreOptions := nodewith.Name("More options").Role(role.PopUpButton)
		changeLayoutItem := nodewith.Name("Change layout").Role(role.MenuItem)
		changeLayoutPanel := nodewith.Name("Change layout").Role(role.Dialog)
		openLayout := uiauto.NamedAction("open layout", ui.Retry(retryTimes, uiauto.Combine("open layout",
			ui.WithTimeout(mediumUITimeout).RetryUntil(ui.DoDefault(moreOptions), ui.WaitUntilExists(changeLayoutItem)),
			ui.RetryUntil(ui.WithTimeout(shortUITimeout).FocusAndWait(changeLayoutItem), ui.Exists(changeLayoutItem.Focused())),
			uiauto.NamedAction("click change layout item", ui.LeftClick(changeLayoutItem)),
			uiauto.NamedAction("wait for change layout panel", ui.WithTimeout(longUITimeout).WaitUntilExists(changeLayoutPanel)),
		)))
		setTiles := func(mode string) action.Action {
			return func(ctx context.Context) error {
				if mode != "Tiled" {
					return nil
				}
				slider := nodewith.Name("Tiles").Role(role.Slider).First()
				clickSliderToMax := func(ctx context.Context) error {
					sliderLocation, err := ui.Location(ctx, slider)
					if err != nil {
						return errors.Wrap(err, "failed to get slider info")
					}
					expectedlocation := coords.Point{X: sliderLocation.Right() - 1, Y: sliderLocation.CenterY()}
					testing.ContextLog(ctx, "Click slider to max")
					return ui.MouseClickAtLocation(0, expectedlocation)(ctx)
				}
				isMaxTiles := func(ctx context.Context) error {
					const expectedResult = "49 tiles"
					sliderInfo, err := ui.Info(ctx, slider)
					if err != nil {
						return errors.Wrap(err, "failed to get slider info")
					}
					value := sliderInfo.Value
					if value == expectedResult {
						testing.ContextLogf(ctx, "Get the expected tiles: got %q", value)
						return nil
					}
					testing.ContextLogf(ctx, "Tiles info: got %q; want %q", value, expectedResult)
					return errors.Errorf("wrong tiles: got %q; want %q", value, expectedResult)
				}
				return ui.Retry(retryTimes, uiauto.Combine("set tiles",
					ui.LeftClick(slider),
					ui.WithInterval(shortUITimeout).RetryUntil(clickSliderToMax, isMaxTiles),
				))(ctx)
			}
		}
		checkTiledGrids := func(mode string) action.Action {
			return func(ctx context.Context) error {
				if mode != "Tiled" {
					return nil
				}
				grids, err := conf.getStableGrids(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to get stable grids")
				}
				// Check classrooms to expect grids to be more than 16 grids.
				if conf.roomSize == ClassRoomSize && len(grids) <= 16 {
					return errors.Wrapf(err, "unexpected grids: got: %v; want more than 16 grids", len(grids))
				}
				return nil
			}
		}
		actionName := "change layout to " + mode
		modeNode := nodewith.Name(mode).Role(role.RadioButton)
		return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
			conf.closeNotifDialog(),
			openLayout,
			uiauto.NamedAction("click layout "+mode,
				ui.WithTimeout(mediumUITimeout).LeftClickUntil(modeNode,
					ui.WithTimeout(shortUITimeout).Exists(modeNode.Focused()))),
			setTiles(mode),
			uiauto.NamedAction("press esc to close layout panel", conf.kb.AccelAction("esc")),
			ui.Retry(5, checkTiledGrids(mode)),
		))(ctx)
	}
}

// BackgroundChange will sequentially change the background to blur, sky picture and turn off background effects.
func (conf *GoogleMeetConference) BackgroundChange(ctx context.Context) error {
	ui := conf.ui
	pinToMainScreen := func(ctx context.Context) error {
		pinBtn := nodewith.NameContaining("Pin yourself").Role(role.Button)
		if err := ui.WaitUntilExists(pinBtn)(ctx); err != nil {
			// If there are no participants in the room, the pin button will not be displayed.
			return ParticipantError(errors.Wrap(err, "failed to find the button to pin to main screen; other participants might have left"))
		}
		return uiauto.NamedAction("to pin to main screen", ui.LeftClick(pinBtn))(ctx)
	}
	changeBackground := func(background string) action.Action {
		moreOptions := nodewith.Name("More options").Role(role.PopUpButton)
		menu := nodewith.Name("Call options").Role(role.Menu)
		// There are two different versions of ui for different accounts to change the background.
		// The old version shows "Change background", the new version shows "Apply visual effects".
		changeBackground := nodewith.NameRegex(regexp.MustCompile("(Apply visual effects|Change background)")).Role(role.MenuItem)
		backgroundButton := nodewith.NameContaining(background).Role(role.ToggleButton)
		webArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
		closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(webArea)
		return uiauto.NamedAction(fmt.Sprintf("change background to %s and enter full screen", background),
			uiauto.Combine("change background and enter full screen",
				ui.Retry(retryTimes, cuj.ExpandMenu(conf.tconn, moreOptions, menu, 433)),
				ui.LeftClick(changeBackground), // Open "Background" panel.
				ui.WithTimeout(mediumUITimeout).LeftClick(backgroundButton),
				ui.LeftClick(closeButton), // Close "Background" panel.
				takeScreenshot(conf.cr, conf.outDir, "change-background-to-"+background),
				// Double click to enter full screen.
				doFullScreenAction(conf.tconn, ui.DoubleClick(webArea), "Meet", true),
				// After applying new background, give it 5 seconds for viewing before applying next one.
				uiauto.Sleep(viewingTime),
				// Double click to exit full screen.
				doFullScreenAction(conf.tconn, ui.DoubleClick(webArea), "Meet", false),
			))
	}
	if err := uiauto.Combine("pin to main screen and change background",
		conf.uiHandler.SwitchToChromeTabByName("Meet"),
		pinToMainScreen,
		changeBackground(staticBackground),
		changeBackground(dynamicBackground),
		changeBackground(blurBackground),
	)(ctx); err != nil {
		return CheckSignedOutError(ctx, conf.tconn, err)
	}
	return nil
}

// Presenting creates Google Slides and Google Docs, shares screen and presents
// the specified application to the conference.
func (conf *GoogleMeetConference) Presenting(ctx context.Context, application googleApplication) (err error) {
	tconn := conf.tconn
	ui := uiauto.New(tconn)
	var appTabName string
	switch application {
	case googleSlides:
		appTabName = slideTabName
	case googleDocs:
		appTabName = docTabName
	}
	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not find the Chrome app")
	}
	switchToTab := func(tabName string) action.Action {
		if conf.extendedDisplay {
			return uiauto.NamedAction("switch window to "+tabName, conf.uiHandler.SwitchToAppWindowByName(chromeApp.Name, tabName))
		}
		return uiauto.NamedAction("switch tab to "+tabName, conf.uiHandler.SwitchToChromeTabByName(tabName))
	}
	meetWebArea := nodewith.NameContaining("Meet").Role(role.RootWebArea)
	alertDialog := nodewith.Name("Your screen is still visible to others").Role(role.Alert)
	closeButton := nodewith.Name("Close").Role(role.Button).Ancestor(alertDialog)
	closeAlertDialog := uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(closeButton),
		uiauto.NamedAction("close alert dialog", ui.LeftClick(closeButton)))

	// shareScreen shares screen by "A Tab" and selects the tab which is going to present.
	// If there is extended display, move conference to extended display.
	shareScreen := func(ctx context.Context) error {
		if conf.roomSize == NoRoom {
			// Share screen will automatically switch to the specified application tab.
			// Without googlemeet, it must switch to slide tab before present slide.
			// And present document doesn't need a switch because it is already on the document page.
			if application == googleSlides {
				return switchToTab(appTabName)(ctx)
			}
			return nil
		}
		ui := uiauto.New(tconn)
		menu := nodewith.Name("Presentation options").Role(role.Menu).Ancestor(meetWebArea)
		presentNowButton := nodewith.Name("Present now").Role(role.PopUpButton).Ancestor(meetWebArea)
		presentMode := nodewith.NameContaining("A tab").Role(role.MenuItem)
		presentTab := nodewith.ClassName("AXVirtualView").Role(role.Cell).Name(appTabName)
		presentingButton := nodewith.NameContaining("presenting").Role(role.PopUpButton)
		shareButton := nodewith.Name("Share").Role(role.Button)
		// There are two "Stop presenting" buttons on the screen with the same ancestor, role and name that we can't use unique finder.
		stopSharing := nodewith.Name("Stop sharing").Role(role.Button).First()

		// If another participant is presenting, wait for the presentation to stop.
		checkPresentNowButton := func(ctx context.Context) error {
			return testing.Poll(ctx, func(ctx context.Context) error {
				if err := ui.WaitUntilExists(presentNowButton)(ctx); err == nil {
					testing.ContextLog(ctx, `"Preset now" button is found`)
					return nil
				}
				if err := ui.Exists(presentingButton)(ctx); err != nil {
					return testing.PollBreak(errors.Wrap(err, `failed to find "Present now" button`))
				}
				testing.ContextLog(ctx, "Another participant is presenting now, wait for the presentation to stop")
				return errors.New("Another participant is presenting now")
			}, &testing.PollOptions{Timeout: longUITimeout})
		}

		testing.ContextLog(ctx, "Share screen")
		return ui.Retry(retryTimes, uiauto.Combine("share screen",
			switchToTab("Meet"),
			checkPresentNowButton,
			cuj.ExpandMenu(conf.tconn, presentNowButton, menu, 113),
			ui.LeftClick(presentMode),
			ui.LeftClick(presentTab),
			ui.LeftClickUntil(shareButton, ui.Gone(shareButton)),
			closeAlertDialog,
			ui.WithTimeout(longUITimeout).WaitUntilExists(stopSharing),
		))(ctx)
	}
	stopPresenting := func(ctx context.Context) error {
		if conf.roomSize == NoRoom {
			return nil
		}
		// There are two "Stop presenting" buttons on the screen with the same ancestor, role and name that we can't use unique finder.
		stopPresentingButton := nodewith.Name("Stop presenting").Role(role.Button).Ancestor(meetWebArea).First()
		testing.ContextLog(ctx, "Stop presenting")
		return uiauto.Combine("stop presenting",
			switchToTab("Meet"),
			closeAlertDialog,
			ui.LeftClickUntil(stopPresentingButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(stopPresentingButton)),
		)(ctx)
	}
	if err := presentApps(ctx, tconn, conf.uiHandler, conf.cr, conf.br, shareScreen, stopPresenting,
		application, conf.outDir, conf.extendedDisplay); err != nil {
		return errors.Wrapf(err, "failed to present %s", string(application))
	}
	return nil
}

// End ends the conference.
func (conf *GoogleMeetConference) End(ctx context.Context) error {
	return cuj.CloseAllWindows(ctx, conf.tconn)
}

// SetBrowser sets browser to chrome or lacros.
func (conf *GoogleMeetConference) SetBrowser(br *browser.Browser) {
	conf.br = br
}

// checkLostNetwork checks for lost network connections.
func (conf *GoogleMeetConference) checkLostNetwork(ctx context.Context) error {
	const lostConnectionText = "You lost your network connection."
	lostConnection := nodewith.NameContaining(lostConnectionText).Role(role.StaticText)
	testing.ContextLog(ctx, "Check for lost network connection")
	if err := conf.ui.WithTimeout(5 * time.Second).WaitUntilExists(lostConnection)(ctx); err == nil {
		testing.ContextLog(ctx, "Lost network message: ", lostConnectionText)
		conf.networkLostCount++
		if err := takeScreenshot(conf.cr, conf.outDir, "lost-connection")(ctx); err == nil {
			testing.ContextLog(ctx, "Take screenshot for lost network connection")
		}
	}
	return nil
}

// LostNetworkCount returns the count of lost network connections.
func (conf *GoogleMeetConference) LostNetworkCount() int {
	return conf.networkLostCount
}

// DisplayAllParticipantsTime returns the loading time for displaying all participants.
func (conf *GoogleMeetConference) DisplayAllParticipantsTime() time.Duration {
	return conf.displayAllParticipantsTime
}

func (conf *GoogleMeetConference) closeNotifDialog() action.Action {
	notiPerm := nodewith.NameContaining("Show notifications").ClassName("RootView").Role(role.AlertDialog)
	allowButton := nodewith.Name("Allow").Role(role.Button).Ancestor(notiPerm)
	// Allow notifications if it popup the dialog.
	return uiauto.IfSuccessThen(conf.ui.Exists(allowButton), conf.ui.LeftClick(allowButton))
}

var _ Conference = (*GoogleMeetConference)(nil)

// NewGoogleMeetConference creates Google Meet conference room instance which implements Conference interface.
func NewGoogleMeetConference(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, uiHandler cuj.UIActionHandler,
	tabletMode, extendedDisplay bool, bt browser.Type, roomSize int, account, password, outDir string) *GoogleMeetConference {
	ui := uiauto.New(tconn)
	return &GoogleMeetConference{
		cr:              cr,
		tconn:           tconn,
		kb:              kb,
		ui:              ui,
		uiHandler:       uiHandler,
		tabletMode:      tabletMode,
		extendedDisplay: extendedDisplay,
		bt:              bt,
		roomSize:        roomSize,
		account:         account,
		password:        password,
		outDir:          outDir,
	}
}
