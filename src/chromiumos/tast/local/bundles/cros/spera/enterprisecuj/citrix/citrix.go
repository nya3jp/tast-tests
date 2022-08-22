// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package citrix contains utils for all operations on Citrix Workspace app.
package citrix

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// WindowsApp defines all apps used in Citrix Workspace app.
type WindowsApp string

const (
	// GoogleChrome defines name of Google Chrome app.
	GoogleChrome WindowsApp = "Google Chrome"
	// TaskManager defines name of Task Manager.
	TaskManager WindowsApp = "Task Manager"
)

var iconInTaskManager = map[WindowsApp]string{
	GoogleChrome: IconChromeTaskManager,
}

// TestMode indicates whether to run in normal/record/replay mode.
type TestMode string

const (
	// ReplayMode represents "replay" mode.
	ReplayMode TestMode = "replay"
	// RecordMode represents "record" mode.
	RecordMode TestMode = "record"
	// NormalMode represents "normal" mode.
	NormalMode TestMode = "normal"
)

const (
	// shortUITimeout used for situations where UI response might be faster.
	shortUITimeout = 3 * time.Second
	// viewingTime used to view the effect after clicking application.
	viewingTime = 2 * time.Second
	// uiWaitTimeFactor used in replay mode, the record waiting time for UI is divided by this number
	// as the time waiting for UI in replay mode.
	uiWaitTimeFactor = 3
	// uiVerifyInterval is used in replay mode, the real ud.WaitUntilExist is called when the number
	// of executions of Fake UI Detects reaches uiVerifyInterval.
	uiVerifyInterval = 5
)

// Citrix defines the struct related to Citrix Workspace app.
type Citrix struct {
	tconn        *chrome.TestConn
	ui           *uiauto.Context
	ud           *uidetection.Context
	kb           *input.KeyboardEventWriter
	dataPath     func(string) string
	desktopTitle string
	appStartTime int64
	tabletMode   bool
	// testMode represents which mode is currently executed.
	// There are three modes: normal, record and replay mode.
	testMode      TestMode
	coordinates   map[string]coords.Point
	uiWaitTime    map[string]int
	uiVerifyCount int
}

// NewCitrix creates an instance of Citrix.
func NewCitrix(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, dataPath func(string) string, desktopTitle string, tabletMode bool, testMode TestMode) *Citrix {
	return &Citrix{
		tconn:        tconn,
		ui:           uiauto.New(tconn),
		ud:           uidetection.NewDefault(tconn),
		kb:           kb,
		dataPath:     dataPath,
		desktopTitle: desktopTitle,
		tabletMode:   tabletMode,
		testMode:     testMode,
		// coordinates and uiWaitTime used in record and replay test mode.
		// In record mode, the coordinates and ui wait times of picture and text detected by uidetection will be
		// recorded to coordinates and uiWaitTime.
		// In replay mode, the coordinates and uiWait file will be loaded to the coordinates and uiWaitTime
		// in order to click and wait for the picture and text.
		coordinates: make(map[string]coords.Point),
		uiWaitTime:  make(map[string]int),
		// uiVerifyCount used in replay mode, it records the number of executions of fake ui detection.
		uiVerifyCount: 0,
	}
}

// Open opens Citrix Workspace app.
func (c *Citrix) Open() action.Action {
	return func(ctx context.Context) error {
		if err := ash.WaitForChromeAppInstalled(ctx, c.tconn, apps.Citrix.ID, 2*time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for Citrix to install")
		}
		startTime := time.Now()
		if err := apps.Launch(ctx, c.tconn, apps.Citrix.ID); err != nil {
			return errors.Wrap(err, "failed to launch Citrix")
		}
		if err := ash.WaitForApp(ctx, c.tconn, apps.Citrix.ID, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait Citrix appear in shelf after launch")
		}
		c.appStartTime = time.Since(startTime).Milliseconds()
		return nil
	}
}

// AppStartTime returns the startup time of Citrix Workspace app in milliseconds.
func (c *Citrix) AppStartTime() int64 {
	return c.appStartTime
}

// Login logs in to Citrix Workspace app.
func (c *Citrix) Login(serverURL, userName, password string) action.Action {
	searchWorkspace := nodewith.Name("Search Workspace").HasClass("citrix-ui__button").Role(role.Button)
	logOnAreaFinder := nodewith.Ancestor(nodewith.HasClass("logon-area").Role(role.LayoutTable))
	editField := logOnAreaFinder.State(state.Editable, true).Role(role.GenericContainer)
	serverURLField := logOnAreaFinder.Name("Store URL or Email address").Role(role.TextField)
	userNameField := logOnAreaFinder.Name("User name:").Role(role.TextField)
	passwordField := logOnAreaFinder.Name("Password:").Role(role.TextField)
	connectBtn := logOnAreaFinder.Name("Connect").HasClass("button").Role(role.Link)
	logOnBtn := logOnAreaFinder.Name("Log On").HasClass("button").Role(role.Link)
	connectServer := uiauto.NamedCombine("connect to Citrix server",
		c.ui.LeftClick(editField.Ancestor(serverURLField)),
		c.kb.TypeAction(serverURL),
		c.ui.DoDefault(connectBtn))
	return uiauto.IfFailThen(c.ui.WithTimeout(shortUITimeout).WaitUntilExists(searchWorkspace),
		uiauto.NamedCombine("login to Citrix",
			uiauto.IfSuccessThen(c.ui.WithTimeout(shortUITimeout).WaitUntilExists(serverURLField), connectServer),
			c.ui.LeftClick(editField.Ancestor(userNameField)),
			c.kb.TypeAction(userName),
			c.ui.LeftClick(editField.Ancestor(passwordField)),
			c.kb.TypeAction(password),
			c.ui.DoDefault(logOnBtn),
			c.ui.WaitUntilExists(searchWorkspace),
		))
}

// Logout logouts Citrix Workspace app.
func (c *Citrix) Logout() action.Action {
	accountLogo := nodewith.Name("C").Role(role.StaticText)
	logOutText := nodewith.Name("Log Out").Role(role.StaticText)
	backToSignIn := nodewith.Name("Back to Sign In").Role(role.Button)
	return uiauto.NamedCombine("logout",
		uiauto.IfFailThen(c.ui.WithTimeout(shortUITimeout).WaitUntilExists(accountLogo), c.kb.AccelAction("Esc")),
		c.ui.LeftClick(accountLogo),
		c.ui.LeftClick(logOutText),
		c.ui.LeftClick(backToSignIn),
	)
}

// ConnectRemoteDesktop connects to the remote desktop by given desktop name.
func (c *Citrix) ConnectRemoteDesktop(desktop string) action.Action {
	uiCtx := uiContext("ConnectRemoteDesktop")
	searchWorkspace := nodewith.Name("Search Workspace").HasClass("citrix-ui__button").Role(role.Button)
	listBoxFinder := nodewith.Ancestor(nodewith.HasClass("ul_vabzqc").Role(role.ListBox))
	listApp := listBoxFinder.Name(desktop).Role(role.ListBoxOption)
	cancelButton := c.customIcon(IconTrackerCancel)
	return uiauto.NamedCombine("connect to remote desktop "+desktop+" in Citrix Workspace",
		c.ui.DoDefault(searchWorkspace),
		c.kb.TypeAction(desktop),
		c.ui.DoDefault(listApp),
		c.waitIcon(uiCtx, IconToolbar),
		uiauto.IfSuccessThen(c.ud.Exists(cancelButton), c.clickIcon(uiCtx, IconTrackerCancel)),
	)
}

// FocusOnDesktop focuses on desktop.
// Sometimes, some actions are performed on chrome os when Citrix Workplace is open without
// focus on it. If there is focus to the desktop, this problem can be solved.
func (c *Citrix) FocusOnDesktop() action.Action {
	uiCtx := uiContext("FocusOnDesktop")
	return c.clickIcon(uiCtx, IconDesktop)
}

// FullscreenDesktop sets Citrix remote desktop to fullscreen.
func (c *Citrix) FullscreenDesktop() action.Action {
	return func(ctx context.Context) error {
		if !c.tabletMode {
			// Wait for the citrix remote desktop window to open.
			if _, err := ash.WaitForAnyWindowWithTitle(ctx, c.tconn, c.desktopTitle); err != nil {
				return errors.Wrap(err, "could not find the citrix remote desktop window")
			}
			window, err := ash.FindWindow(ctx, c.tconn, func(w *ash.Window) bool {
				return w.WindowType == ash.WindowTypeExtension && strings.Contains(w.Title, c.desktopTitle)
			})
			if err != nil {
				return errors.Wrapf(err, "failed to find the %q window", c.desktopTitle)
			}
			if err := ash.SetWindowStateAndWait(ctx, c.tconn, window.ID, ash.WindowStateFullscreen); err != nil {
				// Just log the error and try to continue.
				testing.ContextLogf(ctx, "Try to continue the test even though fullscreen the %q window failed: %v", c.desktopTitle, err)
			}
		}
		return nil
	}
}

// ExitFullscreenDesktop sets Citrix remote desktop to exit fullscreen.
func (c *Citrix) ExitFullscreenDesktop() action.Action {
	return func(ctx context.Context) error {
		if !c.tabletMode {
			// Wait for the citrix remote desktop window to open.
			if _, err := ash.WaitForAnyWindowWithTitle(ctx, c.tconn, c.desktopTitle); err != nil {
				return errors.Wrap(err, "could not find the citrix remote desktop window")
			}
			window, err := ash.FindWindow(ctx, c.tconn, func(w *ash.Window) bool {
				return w.WindowType == ash.WindowTypeExtension && strings.Contains(w.Title, c.desktopTitle)
			})
			if err != nil {
				return errors.Wrapf(err, "failed to find the %q window", c.desktopTitle)
			}
			if err := ash.SetWindowStateAndWait(ctx, c.tconn, window.ID, ash.WindowStateNormal); err != nil {
				// Just log the error and try to continue.
				testing.ContextLogf(ctx, "Try to continue the test even though fullscreen the %q window failed: %v", c.desktopTitle, err)
			}
		}
		return nil
	}
}

// NewTab opens chrome broswer with url in the remote desktop.
func (c *Citrix) NewTab(url string, newWindow bool) action.Action {
	uiCtx := uiContext("NewTab")
	return func(ctx context.Context) error {
		newTabAction := c.kb.AccelAction("Ctrl+T")
		if newWindow {
			newTabAction = func(ctx context.Context) error {
				desktop := c.customIcon(IconDesktop)
				if err := c.ud.Exists(desktop)(ctx); err != nil {
					return c.kb.AccelAction("Ctrl+N")(ctx)
				}
				return c.searchToOpenApplication(GoogleChrome)(ctx)
			}
		}
		return uiauto.NamedCombine("open tab "+url,
			newTabAction,
			c.waitText(uiCtx, "Search Google"),
			c.kb.TypeAction(url),
			c.kb.AccelAction("Enter"),
		)(ctx)
	}
}

// OpenChromeWithURLs opens chrome broswer with urls in the remote desktop.
func (c *Citrix) OpenChromeWithURLs(urls []string) action.Action {
	return func(ctx context.Context) error {
		for i, url := range urls {
			if err := c.NewTab(url, i == 0)(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// searchToOpenApplication searchs and open the application in the remote desktop.
func (c *Citrix) searchToOpenApplication(appName WindowsApp) action.Action {
	uiCtx := uiContext("searchToOpenApplication")
	return uiauto.NamedCombine("search to open application "+string(appName),
		c.clickIcon(uiCtx, IconSearch),
		c.waitText(uiCtx, "Type here to search"),
		c.kb.TypeAction(string(appName)),
		c.clickText(uiCtx, "Desktop app"))
}

// SearchFromWiki searchs from wiki.
func (c *Citrix) SearchFromWiki(text string) action.Action {
	uiCtx := uiContext("SearchFromWiki")
	return uiauto.NamedCombine("search '"+text+"' from wiki",
		c.waitIcon(uiCtx, IconChromeWikiSearch),
		c.kb.TypeAction(text),
		c.kb.AccelAction("Enter"),
		uiauto.Sleep(viewingTime),
	)
}

// SearchFromGoogle searchs from Google.
func (c *Citrix) SearchFromGoogle(text string) action.Action {
	uiCtx := uiContext("SearchFromGoogle")
	return uiauto.NamedCombine("search '"+text+"' from Google",
		c.waitIcon(uiCtx, IconChromeGoogleSearch),
		c.kb.TypeAction(text),
		c.kb.AccelAction("Enter"),
		uiauto.Sleep(viewingTime),
	)
}

// SwitchWindow switches window to next window.
func (c *Citrix) SwitchWindow() action.Action {
	return uiauto.NamedCombine("swich window",
		c.kb.AccelAction("Alt+Esc"),
		uiauto.Sleep(viewingTime),
	)
}

// SwitchTab switches tab to next tab.
func (c *Citrix) SwitchTab() action.Action {
	return uiauto.NamedCombine("swich tab",
		c.kb.AccelAction("Ctrl+Tab"),
		uiauto.Sleep(viewingTime),
	)
}

// CreateGoogleKeepNote opens Google keep and create new note.
func (c *Citrix) CreateGoogleKeepNote(text string) action.Action {
	uiCtx := uiContext("CreateGoogleKeepNote")
	return uiauto.NamedCombine("open google keep and create new note",
		c.NewTab(cuj.GoogleKeepURL, true),
		c.clickText(uiCtx, "Take a note..."),
		c.kb.TypeAction(text),
		c.kb.AccelAction("Esc"), // Save note.
		c.waitText(uiCtx, text),
	)
}

// DeleteGoogleKeepNote deletes note from Google keep.
func (c *Citrix) DeleteGoogleKeepNote(text string) action.Action {
	const retryTimes = 3
	noteText := uidetection.TextBlock(strings.Split(text, " ")).First()
	return uiauto.Retry(retryTimes,
		uiauto.NamedCombine("delete note from google keep",
			c.kb.TypeAction("k"),        // Select note.
			c.kb.AccelAction("Shift+3"), // Delete note.
			c.ud.WithTimeout(5*time.Second).WaitUntilGone(noteText),
		))
}

// UploadPhoto uploads photo to Google photo.
func (c *Citrix) UploadPhoto(filename string) action.Action {
	uiCtx := uiContext("UploadPhoto")
	uploadButton := c.customIcon(IconPhotosUpload)
	downloadButton := c.customIcon(IconPhotosDownload)
	fileFinder := uidetection.Word(filename).Above(uidetection.Word("Cancel"))
	verifiedAndMeasureUploadTime := func(ctx context.Context) error {
		startTime := time.Now()
		if err := c.waitText(uiCtx, "1 item uploaded")(ctx); err != nil {
			return err
		}
		uploadTime := time.Now().Sub(startTime)
		testing.ContextLog(ctx, "Upload photo to Google photo took ", uploadTime)
		return nil
	}

	return uiauto.NamedCombine("upload photo to Google photo",
		c.NewTab(cuj.GooglePhotosURL, true),
		uiauto.IfFailThen(c.ud.WithTimeout(shortUITimeout).LeftClick(uploadButton),
			c.clickIcon(uiCtx, IconPhotosUploadSmall)),
		c.clickIcon(uiCtx, IconPhotosComputer),
		uiauto.IfSuccessThen(c.ud.WithTimeout(shortUITimeout).WaitUntilExists(downloadButton),
			c.clickIcon(uiCtx, IconPhotosDownload)),
		c.clickFinder(uiCtx+filename, fileFinder),
		c.kb.AccelAction("Enter"),
		verifiedAndMeasureUploadTime,
	)
}

// DeletePhoto deletes photo from Google photo.
func (c *Citrix) DeletePhoto() action.Action {
	uiCtx := uiContext("DeletePhoto")
	return uiauto.NamedCombine("delete photo from Google photo",
		c.kb.AccelAction("Right"),
		c.kb.AccelAction("Enter"),
		c.clickIcon(uiCtx, IconPhotosDelete),
		c.clickText(uiCtx, "Move to trash"),
	)
}

// CloseApplication closes application by task mangaer in the remote desktop.
func (c *Citrix) CloseApplication(appName WindowsApp) action.Action {
	uiCtx := uiContext("CloseApplication")
	return uiauto.NamedCombine("close windows application by task mangaer",
		c.searchToOpenApplication(TaskManager),
		c.clickIcon(uiCtx, iconInTaskManager[appName]),
		c.clickIcon(uiCtx, IconEndTask),
		c.kb.AccelAction("Esc"))
}

// CloseAllChromeBrowsers closes all chrome browsers in the remote desktop.
func (c *Citrix) CloseAllChromeBrowsers() action.Action {
	desktop := c.customIcon(IconDesktop)
	chromeActiveIcon := c.customIcon(IconChromeActive)
	return uiauto.IfSuccessThen(c.ud.Gone(desktop),
		uiauto.NamedCombine("close chrome browser",
			c.ud.RightClick(chromeActiveIcon),
			uiauto.Sleep(time.Second), // Sleep to wait for the menu to pop up.
			c.kb.AccelAction("Up"),
			uiauto.Sleep(time.Second), // Sleep to wait to focus on closing option.
			c.kb.AccelAction("Enter"),
			uiauto.Sleep(viewingTime),
		))
}

// Close closes Citrix app and remote desktop.
func (c *Citrix) Close(ctx context.Context) error {
	w, err := ash.GetActiveWindow(ctx, c.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the active window")
	}
	if err := w.CloseWindow(ctx, c.tconn); err != nil {
		return errors.Wrap(err, "failed to close the active window")
	}
	if err := apps.Close(ctx, c.tconn, apps.Citrix.ID); err != nil {
		return errors.Wrap(err, "failed to close Citrix app")
	}
	return nil
}

// SaveRecordFile saves the set of coordinates of finders to the file "coordinates.json" and
// the set of ui wait time to the file "uiwait.json".
func (c *Citrix) SaveRecordFile(ctx context.Context, outDir string) error {
	const (
		coordFileName  = "coordinates.json"
		uiWaitFileName = "uiwait.json"
	)
	filePath := path.Join(outDir, coordFileName)
	j, err := json.MarshalIndent(c.coordinates, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "failed to marshall data for %s json file: %v", coordFileName, c.coordinates)
	}
	if err := ioutil.WriteFile(filePath, j, 0644); err != nil {
		return errors.Wrapf(err, "failed to write %s json file", coordFileName)
	}

	filePath = path.Join(outDir, uiWaitFileName)
	j, err = json.MarshalIndent(c.uiWaitTime, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "failed to marshall data for %s json file: %v", uiWaitFileName, c.uiWaitTime)
	}
	if err := ioutil.WriteFile(filePath, j, 0644); err != nil {
		return errors.Wrapf(err, "failed to write %s json file", uiWaitFileName)
	}
	return nil
}

// LoadRecordFile loads the set of coordinates of finders and the set of ui wait time from the json files.
func (c *Citrix) LoadRecordFile(coordFileName, uiWaitFileName string) error {
	byteValue, err := ioutil.ReadFile(c.dataPath(coordFileName))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(byteValue, &c.coordinates); err != nil {
		return err
	}
	byteValue, err = ioutil.ReadFile(c.dataPath(uiWaitFileName))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(byteValue, &c.uiWaitTime); err != nil {
		return err
	}
	return nil
}

// customIcon returns uidetection finder with file name.
func (c *Citrix) customIcon(name string) *uidetection.Finder {
	return uidetection.CustomIcon(c.dataPath(name))
}

// clickIcon returns action to click the icon.
// The prefix parameter is used to distinguish which function the action is executed in.
// To avoid encountering the situation that different functions have the same picture and text with different coordinates.
func (c *Citrix) clickIcon(prefix, name string) action.Action {
	finder := c.customIcon(name)
	return c.clickFinder(prefix+name, finder)
}

// clickText returns action to click the text.
// The prefix parameter is used to distinguish which function the action is executed in.
// To avoid encountering the situation that different functions have the same picture and text with different coordinates.
func (c *Citrix) clickText(prefix, name string) action.Action {
	splitName := strings.Split(name, " ")
	finder := uidetection.TextBlock(splitName).First()
	return c.clickFinder(prefix+name, finder)
}

// clickFinder returns action to click the finder.
func (c *Citrix) clickFinder(name string, finder *uidetection.Finder) action.Action {
	return func(ctx context.Context) error {
		if c.testMode == RecordMode {
			startTime := time.Now()
			l, err := c.ud.Location(ctx, finder)
			if err != nil {
				return err
			}
			c.uiWaitTime[name] = int(time.Now().Sub(startTime).Milliseconds())
			c.coordinates[name] = l.Rect.CenterPoint()
			testing.ContextLogf(ctx, "Mouse click at %s with location %v", name, l.Rect.CenterPoint())
			return c.ui.MouseClickAtLocation(0, l.Rect.CenterPoint())(ctx)
		} else if c.testMode == ReplayMode {
			empty := coords.Point{}
			if c.coordinates[name] != empty {
				actionName := fmt.Sprintf("mouse click at %s with location %v", name, c.coordinates[name])
				return uiauto.NamedCombine(actionName,
					uiauto.Sleep(time.Duration(float64(c.uiWaitTime[name]/uiWaitTimeFactor)*float64(time.Millisecond))),
					c.ui.MouseClickAtLocation(0, c.coordinates[name]),
				)(ctx)
			}
		}
		return c.ud.LeftClick(finder)(ctx)
	}
}

// waitIcon returns action to wait the icon.
// The prefix parameter is used to distinguish which function the action is executed in.
// To avoid encountering the situation that different functions have the same picture and text with different wait time.
func (c *Citrix) waitIcon(prefix, name string) action.Action {
	finder := c.customIcon(name)
	return c.waitFinder(prefix+name, finder)
}

// waitText returns action to wait the text.
// The prefix parameter is used to distinguish which function the action is executed in.
// To avoid encountering the situation that different functions have the same picture and text with different wait time.
func (c *Citrix) waitText(prefix, name string) action.Action {
	splitName := strings.Split(name, " ")
	finder := uidetection.TextBlock(splitName).First()
	return c.waitFinder(prefix+name, finder)
}

// waitFinder returns action to wait the finder.
func (c *Citrix) waitFinder(name string, finder *uidetection.Finder) action.Action {
	return func(ctx context.Context) error {
		if c.testMode == RecordMode {
			startTime := time.Now()
			if err := c.ud.WaitUntilExists(finder)(ctx); err != nil {
				return err
			}
			c.uiWaitTime[name] = int(time.Now().Sub(startTime).Milliseconds())
			return nil
		} else if c.testMode == ReplayMode {
			// The real ud.WaitUntilExist is called when the number of executions of Fake UI Detects
			// reaches uiVerifyInterval.
			if c.uiVerifyCount%uiVerifyInterval == uiVerifyInterval-1 {
				c.uiVerifyCount = 0
			} else {
				if c.uiWaitTime[name] != 0 {
					c.uiVerifyCount++
					return uiauto.Sleep(time.Duration(float64(c.uiWaitTime[name]/uiWaitTimeFactor) * float64(time.Millisecond)))(ctx)
				}
			}
		}
		return c.ud.WaitUntilExists(finder)(ctx)
	}
}

// uiContext returns the string combination of dash symbol.
func uiContext(name string) string {
	return name + "-"
}
