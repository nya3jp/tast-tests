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
	"chromiumos/tast/local/chrome/browser"
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
	cr                         *chrome.Chrome
	br                         *browser.Browser
	tconn                      *chrome.TestConn
	kb                         *input.KeyboardEventWriter
	ui                         *uiauto.Context
	uiHandler                  cuj.UIActionHandler
	displayAllParticipantsTime time.Duration
	tabletMode                 bool
	roomSize                   int
	networkLostCount           int
	account                    string
	outDir                     string
}

// Zoom has two versions of ui that need to be captured.
const (
	startVideoRegexCapture = "(Start Video|start sending my video|start my video)"
	stopVideoRegexCapture  = "(Stop Video|stop sending my video|stop my video)"
	muteRegexCapture       = "(Mute|mute).*"
	unmuteRegexCapture     = "(Unmute|unmute).*"
	audioRegexCapture      = "(" + muteRegexCapture + "|" + unmuteRegexCapture + ")"
	cameraRegexCapture     = "(" + startVideoRegexCapture + "|" + stopVideoRegexCapture + ")"
)

// Join joins a new conference room.
func (conf *ZoomConference) Join(ctx context.Context, room string, toBlur bool) error {
	ui := conf.ui
	openZoomAndSignIn := func(ctx context.Context) error {
		// Set newWindow to true to launch zoom in the first Chrome tab.
		conn, err := conf.uiHandler.NewChromeTab(ctx, conf.br, cuj.ZoomURL, true)
		if err != nil {
			return errors.Wrap(err, "failed to open the zoom website")
		}
		defer conn.Close()

		if err := webutil.WaitForQuiescence(ctx, conn, longUITimeout); err != nil {
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

		if err := ui.WithTimeout(shortUITimeout).WaitUntilExists(nodewith.Name("MY ACCOUNT").Role(role.Link))(ctx); err != nil {
			testing.ContextLog(ctx, "Start to sign in")
			if err := conn.Navigate(ctx, cuj.ZoomSignInURL); err != nil {
				return err
			}
			account := nodewith.Name(conf.account).First()
			profilePicture := nodewith.Name("Profile picture").First()
			// If the DUT has only one account, it would login to profile page directly.
			// Otherwise, it would show list of accounts.
			if err := uiauto.Combine("sign in",
				uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(account),
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

	// allowPerm allows camera and microphone if browser asks for the permissions.
	allowPerm := func(ctx context.Context) error {
		unableButton := nodewith.NameContaining("Unable to play media.").Role(role.Video)
		// If there is an unable button, it will display a alert dialog to allow permission.
		if err := ui.WithTimeout(shortUITimeout).WaitUntilGone(unableButton)(ctx); err != nil {
			avPerm := nodewith.NameRegex(regexp.MustCompile(".*Use your (microphone|camera).*")).ClassName("RootView").Role(role.AlertDialog).First()
			allowButton := nodewith.Name("Allow").Role(role.Button).Ancestor(avPerm)
			testing.ContextLog(ctx, "Start to allow microphone and camera permissions")
			if err := ui.WaitUntilExists(avPerm)(ctx); err == nil {
				if err := uiauto.Combine("allow microphone and camera",
					// Immediately clicking the allow button sometimes doesn't work. Sleep 2 seconds.
					uiauto.Sleep(2*time.Second),
					ui.LeftClick(allowButton),
					ui.WaitUntilGone(avPerm),
				)(ctx); err != nil {
					return err
				}
			} else {
				testing.ContextLog(ctx, "No action is required to allow microphone and camera")
			}
		}
		return allowPagePermissions(conf.tconn)(ctx)
	}

	// Checks the number of participants in the conference that
	// for different tiers testing would ask for different size
	checkParticipantsNum := func(ctx context.Context) error {
		participant := nodewith.NameContaining("open the participants list pane").Role(role.Button)
		noParticipant := nodewith.NameContaining("[0] particpants").Role(role.Button)
		if err := uiauto.Combine("wait participants",
			ui.WaitUntilExists(participant),
			ui.WithTimeout(mediumUITimeout).WaitUntilGone(noParticipant),
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
		audioButton := nodewith.NameRegex(regexp.MustCompile(audioRegexCapture)).Role(role.Button).Focusable()
		// Not every room will automatically join audio.
		// If there is no automatic join audio, do join audio action.
		if err := ui.WaitUntilExists(audioButton)(ctx); err == nil {
			testing.ContextLog(ctx, "It has automatically joined audio")
			return nil
		}
		joinAudioButton := nodewith.Name("Join Audio by Computer").Role(role.Button)
		testing.ContextLog(ctx, "Join Audio by Computer")
		return ui.WithTimeout(mediumUITimeout).LeftClickUntil(joinAudioButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(joinAudioButton))(ctx)
	}
	startVideo := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Start video")
		cameraButton := nodewith.NameRegex(regexp.MustCompile(cameraRegexCapture)).Role(role.Button)
		startVideoButton := nodewith.NameRegex(regexp.MustCompile(startVideoRegexCapture)).Role(role.Button)
		stopVideoButton := nodewith.NameRegex(regexp.MustCompile(stopVideoRegexCapture)).Role(role.Button)
		// Start video requires camera permission.
		// Allow permission doesn't succeed every time. So add retry here.
		return ui.Retry(3, uiauto.Combine("start video",
			conf.showInterface,
			uiauto.NamedAction("to detect camera button within 15 seconds", ui.WaitUntilExists(cameraButton)),
			// Some DUTs start playing video for the first time.
			// If there is a stop video button, do nothing.
			uiauto.IfSuccessThen(ui.Exists(startVideoButton),
				ui.LeftClickUntil(startVideoButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(startVideoButton))),
			ui.WaitUntilExists(stopVideoButton),
		))(ctx)
	}

	joinButton := nodewith.Name("Join").Role(role.Button)
	webArea := nodewith.NameContaining("Zoom Meeting").Role(role.RootWebArea)
	video := nodewith.Role(role.Video)
	joinFromYourBrowser := nodewith.Name("Join from Your Browser").Role(role.StaticText)
	// There are two types of cookie accept dialogs: "ACCEPT COOKIES" and "ACCEPT ALL COOKIES".
	acceptCookiesButton := nodewith.NameRegex(regexp.MustCompile("ACCEPT.*COOKIES")).Role(role.Button)
	// In Zoom website, the join button may be hidden in tablet mode.
	// Make it visible before clicking.
	// Since ui.MakeVisible() is not always successful, add a retry here.
	clickJoinButton := ui.Retry(3, uiauto.Combine("click join button",
		ui.WaitForLocation(joinButton),
		ui.MakeVisible(joinButton),
		ui.LeftClickUntil(joinButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(joinButton)),
	))
	testing.ContextLog(ctx, "Join conference")
	return uiauto.Combine("join conference",
		openZoomAndSignIn,
		ui.WaitUntilExists(joinFromYourBrowser),
		uiauto.IfSuccessThen(ui.WithTimeout(shortUITimeout).WaitUntilExists(acceptCookiesButton),
			ui.LeftClickUntil(acceptCookiesButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(acceptCookiesButton))),
		ui.LeftClick(joinFromYourBrowser),
		ui.WithTimeout(longUITimeout).WaitUntilExists(joinButton),
		ui.WaitUntilExists(video),
		allowPerm,
		clickJoinButton,
		// Use 1 minute timeout value because it may take longer to wait for page loading,
		// especially for some low end DUTs.
		ui.WithTimeout(longUITimeout).WaitUntilExists(webArea),
		// Sometimes participants number caught at the beginning is wrong, it will be correct after a while.
		// Add retry to get the correct participants number.
		ui.WithInterval(time.Second).Retry(10, checkParticipantsNum),
		ui.Retry(3, joinAudio),
		startVideo,
	)(ctx)
}

// SetLayoutMax sets the conference UI layout to max tiled grid.
func (conf *ZoomConference) SetLayoutMax(ctx context.Context) error {
	return uiauto.Combine("set layout to max",
		conf.changeLayout("Gallery View"),
		uiauto.Sleep(viewingTime), // After applying new layout, give it 5 seconds for viewing before applying next one.
	)(ctx)
}

// SetLayoutMin sets the conference UI layout to minimal tiled grid.
func (conf *ZoomConference) SetLayoutMin(ctx context.Context) error {
	return uiauto.Combine("set layout to minimal",
		conf.changeLayout("Speaker View"),
		uiauto.Sleep(viewingTime), // After applying new layout, give it 5 seconds for viewing before applying next one.
	)(ctx)
}

// changeLayout changes the conference UI layout.
func (conf *ZoomConference) changeLayout(mode string) action.Action {
	return func(ctx context.Context) error {
		ui := conf.ui
		viewButton := nodewith.Name("View").Role(role.Button)
		viewMenu := nodewith.Role(role.Menu).HasClass("dropdown-menu")
		speakerNode := nodewith.Name("Speaker View").Role(role.MenuItem)
		// Sometimes the zoom's menu disappears too fast. Add retry to check whether the device supports
		// speaker and gallery view.
		if err := uiauto.Combine("check view button",
			conf.showInterface,
			ui.LeftClickUntil(viewButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(viewMenu)),
			ui.WithTimeout(shortUITimeout).WaitUntilExists(speakerNode),
		)(ctx); err != nil {
			// Some DUTs don't support 'Speacker View' and 'Gallery View'.
			testing.ContextLog(ctx, "Speaker and Gallery View is not supported on this device, ignore changing the layout")
			return nil
		}

		modeNode := nodewith.Name(mode).Role(role.MenuItem)
		actionName := "Change layout to " + mode
		return uiauto.NamedAction(actionName,
			ui.Retry(3, uiauto.Combine(actionName,
				conf.showInterface,
				uiauto.IfSuccessThen(ui.Gone(modeNode), ui.LeftClick(viewButton)),
				ui.LeftClick(modeNode),
			)))(ctx)
	}
}

// VideoAudioControl controls the video and audio during conference.
func (conf *ZoomConference) VideoAudioControl(ctx context.Context) error {
	ui := conf.ui
	toggleVideo := func(ctx context.Context) error {
		cameraButton := nodewith.NameRegex(regexp.MustCompile(cameraRegexCapture)).Role(role.Button).Focusable()
		info, err := ui.Info(ctx, cameraButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet camera switch button to show")
		}
		startVideoButton := nodewith.NameRegex(regexp.MustCompile(startVideoRegexCapture)).Role(role.Button).Focusable()
		if err := ui.Exists(startVideoButton)(ctx); err == nil {
			testing.ContextLog(ctx, "Turn camera from off to on")
		} else {
			testing.ContextLog(ctx, "Turn camera from on to off")
		}
		nowCameraButton := nodewith.Name(info.Name).Role(role.Button).Focusable()
		if err := ui.LeftClickUntil(nowCameraButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(nowCameraButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle video")
		}
		return nil
	}

	toggleAudio := func(ctx context.Context) error {
		audioButton := nodewith.NameRegex(regexp.MustCompile(audioRegexCapture)).Role(role.Button).Focusable()
		info, err := ui.Info(ctx, audioButton)
		if err != nil {
			return errors.Wrap(err, "failed to wait for the meet microphone switch button to show")
		}
		unmuteButton := nodewith.NameRegex(regexp.MustCompile(unmuteRegexCapture)).Role(role.Button).Focusable()
		if err := ui.Exists(unmuteButton)(ctx); err == nil {
			testing.ContextLog(ctx, "Turn microphone from mute to unmute")
		} else {
			testing.ContextLog(ctx, "Turn microphone from unmute to mute")
		}
		nowAudioButton := nodewith.Name(info.Name).Role(role.Button).Focusable()
		if err := ui.LeftClickUntil(nowAudioButton, ui.WithTimeout(shortUITimeout).WaitUntilGone(nowAudioButton))(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle audio")
		}
		return nil
	}

	return uiauto.Combine("toggle video and audio",
		// Remain in the state for 5 seconds after each action.
		toggleVideo, uiauto.Sleep(viewingTime),
		toggleVideo, uiauto.Sleep(viewingTime),
		toggleAudio, uiauto.Sleep(viewingTime),
		toggleAudio, uiauto.Sleep(viewingTime),
	)(ctx)
}

// SwitchTabs switches the chrome tabs.
func (conf *ZoomConference) SwitchTabs(ctx context.Context) error {
	testing.ContextLog(ctx, "Open wiki page")
	// Set newWindow to false to make the tab in the same Chrome window.
	wikiConn, err := conf.uiHandler.NewChromeTab(ctx, conf.br, cuj.WikipediaURL, false)
	if err != nil {
		return errors.Wrap(err, "failed to open the wiki url")
	}
	defer wikiConn.Close()

	if err := webutil.WaitForQuiescence(ctx, wikiConn, longUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait for wiki page to finish loading")
	}
	return uiauto.Combine("switch tab",
		uiauto.NamedAction("stay wiki page for 3 seconds", uiauto.Sleep(3*time.Second)),
		uiauto.NamedAction("switch to zoom tab", conf.uiHandler.SwitchToChromeTabByName("Zoom")),
	)(ctx)
}

// TypingInChat opens chat window and type.
func (conf *ZoomConference) TypingInChat(ctx context.Context) error {
	const (
		actionName = "open chat window and type"
		message    = "Hello! How are you?"
	)
	chatButton := nodewith.Name("open the chat pane").Role(role.Button)
	chatTextField := nodewith.Name("Type message here ...").Role(role.TextField)
	messageText := nodewith.Name(message).Role(role.StaticText).First()
	manageChatPanel := nodewith.Name("Manage Chat Panel").Role(role.PopUpButton)
	manageChatPanelMenu := nodewith.Name("Manage Chat Panel").Role(role.Menu)
	closeButton := nodewith.Name("Close").Role(role.MenuItem).Ancestor(manageChatPanelMenu)
	return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
		conf.ui.LeftClick(chatButton),
		conf.ui.WaitUntilExists(chatTextField),
		conf.ui.LeftClickUntil(chatTextField, conf.ui.WithTimeout(shortUITimeout).WaitUntilExists(chatTextField.Focused())),
		conf.kb.TypeAction(message),
		conf.kb.AccelAction("enter"),
		conf.ui.WaitUntilExists(messageText),
		uiauto.Sleep(viewingTime), // After typing, wait 5 seconds for viewing.
		conf.ui.LeftClick(manageChatPanel),
		conf.ui.LeftClick(closeButton),
	))(ctx)
}

// BackgroundChange changes the background to patterned background and reset to none.
//
// Zoom doesn't have background blur option for web version so changing background is used to fullfil
// the requirement.
func (conf *ZoomConference) BackgroundChange(ctx context.Context) error {
	ui := conf.ui
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
						ui.WithTimeout(longUITimeout).LeftClickUntil(settingsButton, ui.WaitUntilExists(backgroundTab))))
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
			// Some low end DUTs need more time to load the background settings.
			ui.WithTimeout(longUITimeout).LeftClick(backgroundItem.Nth(backgroundNumber)),
			ui.LeftClick(closeButton), // Close "Background" panel.
			// Double click to enter full screen.
			doFullScreenAction(conf.tconn, ui.DoubleClick(webArea), "Zoom", true),
			// After applying new background, give it 5 seconds for viewing before applying next one.
			uiauto.Sleep(viewingTime),
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
			ui.LeftClickUntil(shareScreenButton, ui.WithTimeout(shortUITimeout).WaitUntilExists(presenMode)),
			ui.LeftClick(presenMode),
			ui.LeftClick(presentTab),
			ui.LeftClick(shareButton),
			ui.WaitUntilExists(stopSharing),
		)(ctx)
	}

	stopPresenting := func(ctx context.Context) error {
		stopSharing := nodewith.Name("Stop sharing").Role(role.Button).First()
		return ui.LeftClickUntil(stopSharing, ui.WithTimeout(shortUITimeout).WaitUntilGone(stopSharing))(ctx)
	}
	// Present on internal display by default.
	presentOnExtendedDisplay := false
	if err := presentApps(ctx, tconn, conf.uiHandler, conf.cr, conf.br, shareScreen, stopPresenting,
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
	ui := conf.ui
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
	}, &testing.PollOptions{Timeout: mediumUITimeout})
}

// SetBrowser sets browser to chrome or lacros.
func (conf *ZoomConference) SetBrowser(br *browser.Browser) {
	conf.br = br
}

// LostNetworkCount returns the count of lost network connections.
func (conf *ZoomConference) LostNetworkCount() int {
	return conf.networkLostCount
}

// DisplayAllParticipantsTime returns the loading time for displaying all participants.
func (conf *ZoomConference) DisplayAllParticipantsTime() time.Duration {
	return conf.displayAllParticipantsTime
}

// NewZoomConference creates Zoom conference room instance which implements Conference interface.
func NewZoomConference(cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter,
	uiHandler cuj.UIActionHandler, tabletMode bool, roomSize int, account, outDir string) *ZoomConference {
	ui := uiauto.New(tconn)
	return &ZoomConference{
		cr:         cr,
		tconn:      tconn,
		kb:         kb,
		ui:         ui,
		uiHandler:  uiHandler,
		tabletMode: tabletMode,
		roomSize:   roomSize,
		account:    account,
		outDir:     outDir,
	}
}
