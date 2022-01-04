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

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
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
	uiHandler  cuj.UIActionHandler
	tabletMode bool
	roomSize   int
	account    string
	outDir     string
}

// Join joins a new conference room.
func (conf *ZoomConference) Join(ctx context.Context, room string, toBlur bool) error {
	ui := uiauto.New(conf.tconn)
	openZoomAndSignIn := func(ctx context.Context) error {
		conn, err := conf.cr.NewConn(ctx, cuj.ZoomURL)
		if err != nil {
			return errors.Wrap(err, "failed to open the zoom website")
		}
		defer conn.Close()

		if err := webutil.WaitForQuiescence(ctx, conn, 45*time.Second); err != nil {
			return errors.Wrapf(err, "failed to wait for %q to be loaded and achieve quiescence", room)
		}
		// Maximize the zoom window to show all the browser UI elements for precise clicking.
		if !conf.tabletMode {
			// Find the zoom browser window.
			window, err := ash.FindWindow(ctx, conf.tconn, func(w *ash.Window) bool {
				return (w.WindowType == ash.WindowTypeBrowser || w.WindowType == ash.WindowTypeLacros) && strings.Contains(w.Title, "Zoom")
			})
			if err != nil {
				return errors.Wrap(err, "failed to find the zoom window")
			}
			if err := ash.SetWindowStateAndWait(ctx, conf.tconn, window.ID, ash.WindowStateMaximized); err != nil {
				// Just log the error and try to continue.
				testing.ContextLog(ctx, "Try to continue the test even though maximizing the zoom window failed: ", err)
			}
		}

		if err := ui.WaitUntilExists(nodewith.Name("SIGN IN").Role(role.Link))(ctx); err == nil {
			testing.ContextLog(ctx, "Start to sign in")
			if err := conn.Navigate(ctx, cuj.ZoomSignInURL); err != nil {
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
		noParticipant := nodewith.NameContaining("[0] particpants").Role(role.Button)
		if err := uiauto.Combine("wait participants",
			ui.WaitUntilExists(participant),
			ui.WithTimeout(30*time.Second).WaitUntilGone(noParticipant),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait participants")
		}
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
	joinAudio := func(ctx context.Context) error {
		audioButton := nodewith.NameRegex(regexp.MustCompile("(mute|unmute) my microphone")).Role(role.Button)
		// Not every room will automatically join audio.
		// If there is no automatic join audio, do join audio action.
		if err := ui.WaitUntilExists(audioButton)(ctx); err == nil {
			testing.ContextLog(ctx, "It has automatically joined audio")
			return nil
		}
		joinAudioButton := nodewith.Name("Join Audio by Computer").Role(role.Button)
		testing.ContextLog(ctx, "Join Audio by Computer")
		return ui.WithTimeout(30*time.Second).LeftClickUntil(joinAudioButton, ui.WithTimeout(time.Second).WaitUntilGone(joinAudioButton))(ctx)
	}

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

	startVideo := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start video")
		cameraButton := nodewith.NameRegex(regexp.MustCompile("(stop|start) sending my video")).Role(role.Button)
		startVideoButton := nodewith.Name("start sending my video").Role(role.Button)
		stopVideoButton := nodewith.Name("stop sending my video").Role(role.Button)
		// Start video requires camera permission.
		// Allow permission doesn't succeed every time. So add retry here.
		return ui.Retry(3, uiauto.Combine("start video",
			allowPerm,
			conf.showInterface,
			uiauto.NamedAction("to detect camera button within 15 seconds", ui.WaitUntilExists(cameraButton)),
			// Some DUTs start playing video for the first time.
			// If there is a stop video button, do nothing.
			ui.IfSuccessThen(ui.Exists(startVideoButton),
				ui.LeftClickUntil(startVideoButton, ui.WithTimeout(time.Second).WaitUntilGone(startVideoButton))),
			ui.WaitUntilExists(stopVideoButton),
		))(ctx)
	}

	joinButton := nodewith.Name("Join").Role(role.Button)
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)
	joinFromYourBrowser := nodewith.Name("Join from Your Browser").Role(role.StaticText)
	// There are two types of cookie accept dialogs: "ACCEPT COOKIES" and "ACCEPT ALL COOKIES".
	acceptCookiesButton := nodewith.NameRegex(regexp.MustCompile("ACCEPT.*COOKIES")).Role(role.Button)
	// In Zoom website, the join button may be hidden in tablet mode.
	// Make it visible before clicking.
	// Since ui.MakeVisible() is not always successful, add a retry here.
	clickJoinButton := ui.Retry(3, uiauto.Combine("click join button",
		ui.WaitForLocation(joinButton),
		ui.MakeVisible(joinButton),
		ui.LeftClickUntil(joinButton, ui.WithTimeout(time.Second).WaitUntilGone(joinButton)),
	))
	testing.ContextLog(ctx, "Join conference")
	return uiauto.Combine("join conference",
		openZoomAndSignIn,
		ui.WaitUntilExists(joinFromYourBrowser),
		ui.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(acceptCookiesButton),
			ui.LeftClickUntil(acceptCookiesButton, ui.WithTimeout(time.Second).WaitUntilGone(acceptCookiesButton))),
		ui.LeftClick(joinFromYourBrowser),
		ui.WithTimeout(time.Minute).WaitUntilExists(joinButton),
		clickJoinButton,
		// Use 1 minute timeout value because it may take longer to wait for page loading,
		// especially for some low end DUTs.
		ui.WithTimeout(time.Minute).WaitUntilExists(webArea),
		// Sometimes participants number caught at the beginning is wrong, it will be correct after a while.
		// Add retry to get the correct participants number.
		ui.WithInterval(time.Second).Retry(10, checkParticipantsNum),
		joinAudio,
		// Launch Meeting page is useless so close it.
		closeLaunchMeetingTab,
		startVideo,
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
		nowCameraButton := nodewith.Name(info.Name).Role(role.Button)
		if strings.HasPrefix(info.Name, "start") {
			testing.ContextLog(ctx, "Turn camera from off to on")
		} else {
			testing.ContextLog(ctx, "Turn camera from on to off")
		}
		if err := ui.LeftClickUntil(nowCameraButton, ui.WithTimeout(5*time.Second).WaitUntilGone(nowCameraButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle video")
		}
		return nil
	}

	toggleAudio := func(ctx context.Context) error {
		microphoneButton := nodewith.NameRegex(regexp.MustCompile("(mute|unmute) my microphone")).Role(role.Button)

		info, err := ui.Info(ctx, microphoneButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet microphone switch button to show")
		}
		nowMicrophoneButton := nodewith.Name(info.Name).Role(role.Button)
		if strings.HasPrefix(info.Name, "unmute") {
			testing.ContextLog(ctx, "Turn microphone from mute to unmute")
		} else {
			testing.ContextLog(ctx, "Turn microphone from unmute to mute")
		}
		if err := ui.LeftClickUntil(nowMicrophoneButton, ui.WithTimeout(5*time.Second).WaitUntilGone(nowMicrophoneButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle audio")
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
	wikiConn, err := conf.cr.NewConn(ctx, cuj.WikipediaURL)
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
	speakerNode := nodewith.Name(speaker).Role(role.MenuItem)
	// Sometimes the zoom's menu disappears too fast. Add retry to check whether the device supports
	// speaker and gallery view.
	if err := ui.Retry(3, uiauto.Combine("check view button",
		conf.showInterface,
		ui.WithTimeout(time.Second).LeftClick(viewButton),
		ui.WithTimeout(time.Second).WaitUntilExists(speakerNode),
	))(ctx); err != nil {
		// Some DUTs don't support 'Speacker View' and 'Gallery View'.
		testing.ContextLog(ctx, "Speaker and Gallery View is not supported on this device, ignore changing the layout")
		return nil
	}

	for _, mode := range []string{speaker, gallery} {
		modeNode := nodewith.Name(mode).Role(role.MenuItem)
		selectMode := func(ctx context.Context) error {
			return uiauto.Combine("select layout mode",
				conf.showInterface,
				ui.LeftClick(viewButton),
				ui.LeftClick(modeNode),
			)(ctx)
		}
		testing.ContextLogf(ctx, "Change layout to %q", mode)
		if err := uiauto.Combine("change layout to '"+mode+"'",
			ui.Retry(3, selectMode),
			ui.Sleep(10*time.Second), //After applying new layout, give it 10 seconds for viewing before applying next one.
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// BackgroundChange changes the background to patterned background and reset to none.
//
// Zoom doesn't have background blur option for web version so changing background is used to fullfil
// the requirement.
func (conf *ZoomConference) BackgroundChange(ctx context.Context) error {
	ui := uiauto.New(conf.tconn)
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)
	changeBackground := func(backgroundNumber int) error {
		settingsButton := nodewith.Name("Settings").Role(role.Button).Ancestor(webArea)
		settingsWindow := nodewith.Name("settings dialog window").Role(role.Application).Ancestor(webArea)
		backgroundTab := nodewith.Name("Background").Role(role.Tab).Ancestor(settingsWindow)
		backgroundItem := nodewith.Role(role.ListItem).Ancestor(settingsWindow)
		closeButton := nodewith.Role(role.Button).HasClass("settings-dialog__close").Ancestor(settingsWindow)
		openBackgroundPanel := func(ctx context.Context) error {
			var actions []action.Action
			if err := conf.showInterface(ctx); err != nil {
				return err
			}
			if err := ui.Exists(settingsButton)(ctx); err == nil {
				actions = append(actions,
					uiauto.NamedAction("click settings button",
						ui.LeftClickUntil(settingsButton, ui.WithTimeout(5*time.Second).WaitUntilExists(backgroundTab))))
			} else {
				// If the screen width is not enough, the settings button will be moved to more options.
				moreOptions := nodewith.Name("More meeting control").Ancestor(webArea)
				moreSettingsButton := nodewith.Name("Settings").Role(role.MenuItem).Ancestor(webArea)
				actions = append(actions,
					uiauto.NamedAction("click more option", ui.LeftClick(moreOptions)),
					uiauto.NamedAction("click settings menu item", ui.LeftClick(moreSettingsButton)),
				)
			}
			actions = append(actions, ui.LeftClick(backgroundTab))
			if err := uiauto.Combine("open background panel", actions...)(ctx); err != nil {
				return errors.Wrap(err, "failed to background panel")
			}
			return nil
		}
		testing.ContextLogf(ctx, "Change background to listitem %d and enter full screen", backgroundNumber)
		return uiauto.Combine("change background",
			ui.Retry(3, openBackgroundPanel), // Open "Background" panel.
			ui.LeftClick(backgroundItem.Nth(backgroundNumber)),
			ui.LeftClick(closeButton), // Close "Background" panel.
			// Double click to enter full screen.
			doFullScreenAction(conf.tconn, ui.DoubleClick(webArea), "Zoom", true),
			// After applying new background, give it 5 seconds for viewing before applying next one.
			ui.Sleep(5*time.Second),
			// Double click to exit full screen.
			doFullScreenAction(conf.tconn, ui.DoubleClick(webArea), "Zoom", false),
		)(ctx)
	}
	if err := conf.uiHandler.SwitchToChromeTabByName("Zoom")(ctx); err != nil {
		return CheckSignedOutError(ctx, conf.tconn, errors.Wrap(err, "failed to switch to zoom page"))
	}
	// Background item doesn't have a specific node name but a role name.
	// We could get the background item from the listitem.
	// The first background item is none, others are patterned background.
	// Click backgroundItem.Nth(1) means change background to first background img.
	// Click backgroundItem.Nth(0) means change background to none.
	if err := changeBackground(1); err != nil {
		return errors.Wrap(err, "failed to change background to first image")
	}
	if err := changeBackground(0); err != nil {
		return errors.Wrap(err, "failed to change background to none")
	}

	return nil
}

// Presenting creates Google Slides and Google Docs, shares screen and presents
// the specified application to the conference.
func (conf *ZoomConference) Presenting(ctx context.Context, application googleApplication) (err error) {
	tconn := conf.tconn
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	var appTabName string
	switch application {
	case googleSlides:
		appTabName = slideTabName
	case googleDocs:
		appTabName = docTabName
	}
	// shareScreen shares screen by "Chrome Tab" and selects the tab which is going to present.
	shareScreen := func(ctx context.Context) error {
		shareScreenButton := nodewith.Name("Share Screen").Role(role.StaticText)
		presenMode := nodewith.Name("Chrome Tab").Role(role.Tab).ClassName("Tab")
		presentTab := nodewith.ClassName("AXVirtualView").Role(role.Cell).Name(appTabName)
		shareButton := nodewith.Name("Share").Role(role.Button)
		stopSharing := nodewith.Name("Stop sharing").Role(role.Button).First()
		testing.ContextLog(ctx, "Share screen")
		return uiauto.Combine("share Screen",
			conf.uiHandler.SwitchToChromeTabByName("Zoom"),
			conf.showInterface,
			ui.LeftClickUntil(shareScreenButton, ui.WithTimeout(time.Second).WaitUntilExists(presenMode)),
			ui.LeftClick(presenMode),
			ui.LeftClick(presentTab),
			ui.LeftClick(shareButton),
			ui.WaitUntilExists(stopSharing),
		)(ctx)
	}

	stopPresenting := func(ctx context.Context) error {
		stopSharing := nodewith.Name("Stop sharing").Role(role.Button).First()
		return ui.LeftClickUntil(stopSharing, ui.WithTimeout(3*time.Second).WaitUntilGone(stopSharing))(ctx)
	}
	// Present on internal display by default.
	presentOnExtendedDisplay := false
	if err := presentApps(ctx, tconn, conf.uiHandler, conf.cr, shareScreen, stopPresenting,
		application, conf.outDir, presentOnExtendedDisplay); err != nil {
		return errors.Wrapf(err, "failed to present %s", string(application))
	}
	return nil
}

// End ends the conference.
func (conf *ZoomConference) End(ctx context.Context) error {
	return cuj.CloseAllWindows(ctx, conf.tconn)
}

var _ Conference = (*ZoomConference)(nil)

// showInterface moves mouse or taps in web area in order to make the menu interface reappear.
func (conf *ZoomConference) showInterface(ctx context.Context) error {
	ui := uiauto.New(conf.tconn)
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)
	information := nodewith.Name("Meeting information").Role(role.Button).Ancestor(webArea)

	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.Exists(information)(ctx); err == nil {
			return nil
		}

		if conf.tabletMode {
			testing.ContextLog(ctx, "Tap web area to show interface")
			if err := conf.uiHandler.Click(webArea)(ctx); err != nil {
				return errors.Wrap(err, "failed to click the web area")
			}
		} else {
			testing.ContextLog(ctx, "Mouse move to show interface")
			webAreaInfo, err := ui.Info(ctx, webArea)
			if err != nil {
				return err
			}
			if err := mouse.Move(conf.tconn, webAreaInfo.Location.TopLeft(), 200*time.Millisecond)(ctx); err != nil {
				return errors.Wrap(err, "failed to move mouse to top left corner of the web area")
			}
			if err := ui.MouseMoveTo(webArea, 200*time.Millisecond)(ctx); err != nil {
				return errors.Wrap(err, "failed to move mouse to the center of the web area")
			}
		}

		if err := ui.WaitUntilExists(information)(ctx); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// NewZoomConference creates Zoom conference room instance which implements Conference interface.
func NewZoomConference(cr *chrome.Chrome, tconn *chrome.TestConn, uiHandler cuj.UIActionHandler, tabletMode bool,
	roomSize int, account, outDir string) *ZoomConference {
	return &ZoomConference{
		cr:         cr,
		tconn:      tconn,
		uiHandler:  uiHandler,
		tabletMode: tabletMode,
		roomSize:   roomSize,
		account:    account,
		outDir:     outDir,
	}
}
