// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"math"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// It may took longer to wait for UI nodes to be stable, especially for some low end DUT.
var defaultPollOpt = testing.PollOptions{Timeout: time.Minute, Interval: time.Second}

// SwitchWindowOption defines types of operations of switching window.
type SwitchWindowOption int

const (
	// SwitchWindowThroughHotseat specifies switch window through hotseat.
	SwitchWindowThroughHotseat SwitchWindowOption = iota
	// SwitchWindowThroughOverview specifies switch window through overview.
	SwitchWindowThroughOverview
	// SwitchWindowThroughKeyEvent specifies switch window through ker event.
	SwitchWindowThroughKeyEvent
	// SwitchWindowThroughShelf specifies switch window through shelf.
	SwitchWindowThroughShelf = SwitchWindowThroughHotseat
)

// UIActionHandler defines UI actions performed either on a tablet or on a clamshell UI.
type UIActionHandler interface {
	// LaunchChrome launches the Chrome browser.
	LaunchChrome(ctx context.Context) (time.Time, error)
	// NewChromeTab creates a new tab of Google Chrome.
	NewChromeTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error)

	// SwitchWindow switches to the next window by key event.
	SwitchWindow(ctx context.Context) error
	// SwitchToLRUWindow switches the window to LRU (Least Recently Used) one.
	// opt specifies the way of switching.
	SwitchToLRUWindow(ctx context.Context, opt SwitchWindowOption) error

	// SwitchToAppWindow switches to the window of the given app.
	// If the APP has multiple windows, it will switch to the first window.
	SwitchToAppWindow(ctx context.Context, appName string) error
	// SwitchToAppWindowByIndex switches to the specific window identified by the window index of the given APP.
	// It is used when the APP has multiple windows.
	SwitchToAppWindowByIndex(ctx context.Context, appName string, targetIdx int) error
	// SwitchToAppWindowByName switches to the specific window identified by the window name of the given APP.
	// It is used when the APP has multiple windows.
	SwitchToAppWindowByName(ctx context.Context, appName, targetName string) error

	// SwitchChromeTab switches to the next Chrome tab by key event.
	SwitchChromeTab(ctx context.Context) error

	// SwitchToChromeTabByIndex switches to the tab identified by the tab index in the current chrome window.
	SwitchToChromeTabByIndex(ctx context.Context, tabIdxDest int) error
	// SwitchToChromeTabByName switches to the tab identified by the tab name in the current chrome window.
	SwitchToChromeTabByName(ctx context.Context, tabNameDest string) error

	// ScrollChromePage generate the scroll actions.
	ScrollChromePage(ctx context.Context) ([]func(ctx context.Context, conn *chrome.Conn) error, error)
	// RefreshChromePage refresh a web page (current focus page).
	RefreshChromePage(ctx context.Context) error

	// MinimizeAllWindow minimizes all window.
	MinimizeAllWindow(ctx context.Context) error
	// Close releases the underlying resouses.
	Close()
}

// TabletActionHandler defines the action on tablet devices.
type TabletActionHandler struct {
	tconn *chrome.TestConn
	ui    *uiauto.Context
	kb    *input.KeyboardEventWriter // Even in tablet mode, some tests might want to use keyboard shortcuts for certain operations.
	tc    *touch.Context
	tew   *input.TouchscreenEventWriter
	tcc   *input.TouchCoordConverter
	stew  *input.SingleTouchEventWriter
}

// NewTabletActionHandler returns the action handler which is responsible for handling UI actions on tablet.
func NewTabletActionHandler(ctx context.Context, tconn *chrome.TestConn) (*TabletActionHandler, error) {
	tc, err := touch.New(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the touch context instance")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the keyboard, error")
	}

	// Get touch controller for tablet.
	tew, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the touchscreen and converter")
	}

	stew, err := tew.NewSingleTouchWriter()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the single touch writer")
	}
	return &TabletActionHandler{
		tconn: tconn,
		kb:    kb,
		ui:    uiauto.New(tconn).WithPollOpts(defaultPollOpt),
		tc:    tc.WithPollOpts(defaultPollOpt),
		tew:   tew,
		tcc:   tcc,
		stew:  stew,
	}, nil
}

// Close releases the underlying resouses.
func (t *TabletActionHandler) Close() {
	t.kb.Close()
	t.stew.Close()
	t.tc.Close()
}

// LaunchChrome launches the Chrome browser.
func (t *TabletActionHandler) LaunchChrome(ctx context.Context) (time.Time, error) {
	return t.clickChromeOnHotseat(ctx)
}

func (t *TabletActionHandler) clickChromeOnHotseat(ctx context.Context) (time.Time, error) {
	return LaunchAppFromHotseat(ctx, t.tconn, "Chrome", "Chromium")
}

// showTabList shows the tab list by clicking a button on the Chrome tool bar.
func (t *TabletActionHandler) showTabList(ctx context.Context) error {
	toggle := nodewith.NameContaining("toggle tab strip").Role(role.Button).First()
	return t.tc.Tap(toggle)(ctx)
}

// NewChromeTab creates a new tab of Google Chrome.
// newWindow indicates whether this new tab should open in current Chrome window or in new Chrome window.
func (t *TabletActionHandler) NewChromeTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if newWindow {
		return cr.NewConn(ctx, url, cdputil.WithNewWindow())
	}

	if err := t.showTabList(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open the tab list")
	}

	btn := nodewith.Name("New tab").Role(role.Button).First()
	if err := t.tc.Tap(btn)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find and click new tab button")
	}

	c, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find new tab")
	}
	if err = c.Navigate(ctx, url); err != nil {
		return c, errors.Wrapf(err, "failed to navigate to %s, error: %v", url, err)
	}

	return c, nil
}

// SwitchWindow switches to the next window by key event.
func (t *TabletActionHandler) SwitchWindow(ctx context.Context) error {
	return t.kb.Accel(ctx, "Alt+Tab")
}

// SwitchToAppWindow switches to the window of the given app.
// If the APP has multiple windows, it will switch to the first window.
func (t *TabletActionHandler) SwitchToAppWindow(ctx context.Context, appName string) error {
	return t.SwitchToAppWindowByIndex(ctx, appName, 0)
}

// SwitchToAppWindowByIndex switches to the specific window identified by the window index of the given APP.
// It is used when the APP has multiple windows.
func (t *TabletActionHandler) SwitchToAppWindowByIndex(ctx context.Context, appName string, targetIdx int) error {
	testing.ContextLogf(ctx, "Switching to app window, by index (%d)", targetIdx)
	// The first one (which is the name of the app) should be skipped.
	menuItem := nodewith.ClassName("MenuItemView").Nth(targetIdx + 1)
	return t.switchToWindowThroughHotseat(ctx, appName, menuItem)
}

// SwitchToAppWindowByName switches to the specific window identified by the window name of the given APP.
// It is used when the APP has multiple windows.
func (t *TabletActionHandler) SwitchToAppWindowByName(ctx context.Context, appName, targetName string) error {
	testing.ContextLogf(ctx, "Switching to app %s window, by name (%s)", appName, targetName)
	menuItem := nodewith.ClassName("MenuItemView").NameContaining(targetName)
	return t.switchToWindowThroughHotseat(ctx, appName, menuItem)
}

// SwitchToLRUWindow switches the window to LRU (Least Recently Used) one.
// opt specifies the way of switching.
func (t *TabletActionHandler) SwitchToLRUWindow(ctx context.Context, opt SwitchWindowOption) error {
	msg := ""
	switch opt {
	case SwitchWindowThroughOverview:
		testing.ContextLog(ctx, "Switching to LRU window, by overview")
		return t.switchToLRUWindowByOverview(ctx)
	case SwitchWindowThroughHotseat:
		msg = "switch to LRU window through hotseat not support for TabletActionHandler, please use SwitchWindowThroughOverview instead"
	case SwitchWindowThroughKeyEvent:
		msg = "switch to LRU window through key event not support for TabletActionHandler, please use SwitchWindowThroughOverview instead"
	}
	return errors.New(msg)
}

// switchToWindowThroughHotseat switch current focus window to another through hotseat.
func (t *TabletActionHandler) switchToWindowThroughHotseat(ctx context.Context, appName string, menuItemFinder *nodewith.Finder) error {
	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, t.tconn, t.stew, t.tcc); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	if strings.Contains(appName, "Chrome") || strings.Contains(appName, "Chromium") {
		if _, err := t.clickChromeOnHotseat(ctx); err != nil {
			return errors.Wrap(err, "failed to clicl Chrome app icon on hotseat")
		}
	} else {
		icon, err := openedAppIconFinder(ctx, t.tconn, appName)
		if err != nil {
			return errors.Wrap(err, "failed to find app icon")
		}
		if err := t.tc.Tap(icon)(ctx); err != nil {
			return errors.Wrap(err, "failed to tap app icon on hotseat")
		}
	}

	if err := t.ui.Exists(nodewith.ClassName("MenuItemView").First())(ctx); err != nil {
		// Node (any menu item) does not exist.
		// In this case, there is only one window for target app, and the window is already switched after tap the icon,
		// so no need to further tap the menu item.
		return nil
	}

	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, t.tconn, ash.ShelfExtended); err != nil {
		return errors.Wrap(err, "failed to wait for hotseat animating to ideal")
	}

	if err := t.tc.Tap(menuItemFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to tap menu item")
	}

	return ash.WaitForHotseatAnimatingToIdealState(ctx, t.tconn, ash.ShelfHidden)
}

// switchToLRUWindowByOverview switches the window to least recently used one through overview.
func (t *TabletActionHandler) switchToLRUWindowByOverview(ctx context.Context) error {
	// Ensure overview is shown.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.DragToShowOverview(ctx, t.tew, t.stew, t.tconn)
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to show overview")
	}
	// Ensure overview is hidden even if an error occur.
	defer ash.SetOverviewModeAndWait(ctx, t.tconn, false)

	targetWindowFinder, err := overviewLRUWindowFinder(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to get LRU window finder")
	}

	return uiauto.Combine("Click item on overview",
		t.tc.Tap(targetWindowFinder),
		t.ui.WaitUntilGone(targetWindowFinder),
	)(ctx)
}

// SwitchChromeTab switches to the next Chrome tab.
func (t *TabletActionHandler) SwitchChromeTab(ctx context.Context) error {
	testing.ContextLog(ctx, "Switching Chrome tab by key event Ctrl+Tab")
	return t.kb.Accel(ctx, "Ctrl+Tab")
}

// SwitchToChromeTabByIndex switches to the tab identified by the tab index in the current chrome window.
func (t *TabletActionHandler) SwitchToChromeTabByIndex(ctx context.Context, tabIdxDest int) error {
	testing.ContextLogf(ctx, "Switching Chrome tab, by index (%d)", tabIdxDest)
	// Open tab list.
	if err := t.showTabList(ctx); err != nil {
		return errors.Wrap(err, "failed to open the tab list")
	}
	return t.switchChromeTab(ctx, tabIdxDest)
}

// SwitchToChromeTabByName switches the Chrome tab to the one with the given name through UI operation.
// The tab name must exact match.
func (t *TabletActionHandler) SwitchToChromeTabByName(ctx context.Context, tabNameDest string) error {
	testing.ContextLogf(ctx, "Switching Chrome tab, by name (%s)", tabNameDest)
	// Open tab list.
	if err := t.showTabList(ctx); err != nil {
		return errors.Wrap(err, "failed to open the tab list")
	}

	// Convert the index of tabs according to given tab name.
	tcFinder := nodewith.Role(role.TabList).Ancestor(nodewith.Role(role.RootWebArea).Name("Tab list"))
	tbFinder := nodewith.Role(role.Tab).Ancestor(tcFinder)
	// Find tab items under tabListContainer.
	tabs, err := t.ui.NodesInfo(ctx, tbFinder)
	if err != nil {
		return errors.Wrap(err, "failed to find tab items in current window")
	}
	tabIdxDest := -1
	for i, tab := range tabs {
		if tab.Name == tabNameDest {
			tabIdxDest = i
			break
		}
	}

	if tabIdxDest == -1 {
		return errors.Errorf("failed to find destination tab with name %q", tabNameDest)
	}

	return t.switchChromeTab(ctx, tabIdxDest)
}

// switchChromeTab switches the Chrome tab to another one identified by tab index in the current chrome window.
func (t *TabletActionHandler) switchChromeTab(ctx context.Context, tabIdxDest int) error {
	// With WebUITabStrip, there would be only one Chrome window at a time.
	tcFinder := nodewith.Role(role.TabList).Ancestor(nodewith.Role(role.RootWebArea).Name("Tab list"))
	tcLocation, err := t.ui.Location(ctx, tcFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get tab list node location")
	}

	tbFinder := nodewith.Role(role.Tab).Ancestor(tcFinder)
	// Find tab items under tabListContainer.
	tabs, err := t.ui.NodesInfo(ctx, tbFinder)
	if err != nil {
		return errors.Wrap(err, "failed to find tab items in current window")
	}

	var (
		swipeDistance    int
		onscreenTabWidth int
		succ             = false
	)
	tabIdxSrc := -1
	for i := 0; i < len(tabs); i++ {
		selected, ok := tabs[i].HTMLAttributes["aria-selected"]
		if !ok {
			return errors.New("tab node doesn't have aria-selected HTML attribute")
		}
		if selected == "true" {
			tabIdxSrc = i
			break
		}
	}
	if tabIdxSrc == -1 {
		return errors.New("failed to find the current selected tab")
	}
	testing.ContextLogf(ctx, "Switch Chrome tab from index %d to %d", tabIdxSrc, tabIdxDest)
	// Find two adjacent items which are both fully in-screen to calculate the swipe distance.
	for i := 0; i < len(tabs)-1; i++ {
		onscreen1 := !tabs[i].State[state.Offscreen]
		onscreen2 := !tabs[i+1].State[state.Offscreen]
		width1 := tabs[i].Location.Width
		width2 := tabs[i+1].Location.Width
		if onscreen1 && onscreen2 && width1 == width2 {
			swipeDistance = tabs[i+1].Location.CenterPoint().X - tabs[i].Location.CenterPoint().X
			onscreenTabWidth = width1
			succ = true
			break
		}
	}
	if !succ {
		return errors.Wrap(err, "failed to find two adjacent tab items within screen")
	}

	// Check if swipe is required to show the target tab.
	if tabs[tabIdxDest].State[state.Offscreen] || tabs[tabIdxDest].Location.Width < onscreenTabWidth {
		swipeDirection := 1 // The direction of swipe. Default is right swipe.
		if tabIdxDest < tabIdxSrc {
			// Left swipe.
			swipeDirection = -1
		}
		swipeDistance *= swipeDirection

		var (
			swipeTimes = int(math.Abs(float64(tabIdxDest - tabIdxSrc)))
			ptSrc      = tcLocation.CenterPoint()
			ptEnd      = coords.NewPoint(ptSrc.X+swipeDistance, ptSrc.Y)
		)

		// The total swipe distance might be greater than screen size, which means the destination point might be out of screen.
		// Make multiple swipes in this case.
		var actions []action.Action
		for i := 0; i < swipeTimes; i++ {
			actions = append(actions, t.tc.Swipe(ptEnd, t.tc.SwipeTo(ptSrc, 500*time.Millisecond)))
		}
		if err := action.Combine("scroll by multipe swipes", actions...)(ctx); err != nil {
			return errors.Wrap(err, "failed to swipe")
		}

		// Wait location be stable after scroll.
		if err := t.ui.WaitForLocation(tbFinder.Nth(tabIdxDest))(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for tab list container to be stable")
		}
		testing.ContextLog(ctx, "Scroll complete, ready for tab switch")
	}

	return t.tc.Tap(tbFinder.Nth(tabIdxDest))(ctx)
}

// RefreshChromePage refresh a web page (current focus page).
func (t *TabletActionHandler) RefreshChromePage(ctx context.Context) error {
	btn := nodewith.Name("Reload").Role(role.Button).ClassName("ReloadButton").First()
	return t.tc.Tap(btn)(ctx)
}

// ScrollChromePage generate the scroll action.
func (t *TabletActionHandler) ScrollChromePage(ctx context.Context) ([]func(ctx context.Context, conn *chrome.Conn) error, error) {
	info, err := display.GetInternalInfo(ctx, t.tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get display info")
	}
	var (
		x      = info.Bounds.Width / 2
		ystart = info.Bounds.Height / 4 * 3           // 75% of screen height.
		yend   = info.Bounds.Height / 4               // 25% of screen height.
		start  = coords.NewPoint(int(x), int(ystart)) // start point of swipe.
		end    = coords.NewPoint(int(x), int(yend))   // end point of swipe.
	)

	// Swipe the page down.
	swipeDown := func(ctx context.Context, conn *chrome.Conn) error {
		if err := t.tc.Swipe(start, t.tc.SwipeTo(end, 500*time.Millisecond))(ctx); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		return nil
	}

	// Swipe the page up.
	swipeUp := func(ctx context.Context, conn *chrome.Conn) error {
		if err := t.tc.Swipe(end, t.tc.SwipeTo(start, 500*time.Millisecond))(ctx); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		return nil
	}

	return []func(ctx context.Context, conn *chrome.Conn) error{
		swipeDown,
		swipeUp,
		swipeUp,
	}, nil
}

// MinimizeAllWindow minimizes all window.
func (t *TabletActionHandler) MinimizeAllWindow(ctx context.Context) error {
	if err := ash.DragToShowHomescreen(ctx, t.tew.Width(), t.tew.Height(), t.stew, t.tconn); err != nil {
		return errors.Wrap(err, "failed to show homescreen")
	}
	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, t.tconn, ash.ShelfShownHomeLauncher); err != nil {
		return errors.Wrap(err, "hotseat is in an unexpected state")
	}
	testing.ContextLog(ctx, "All windows are minimized")
	return nil
}

// ClamshellActionHandler define the action on clamshell devices.
type ClamshellActionHandler struct {
	tconn    *chrome.TestConn
	ui       *uiauto.Context
	kb       *input.KeyboardEventWriter
	pad      *input.TrackpadEventWriter
	touchPad *input.TouchEventWriter
}

// NewClamshellActionHandler returns the action handler which is responsible for handling UI actions on clamshell.
func NewClamshellActionHandler(ctx context.Context, tconn *chrome.TestConn) (*ClamshellActionHandler, error) {
	pad, err := input.Trackpad(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create trackpad event writer")
	}

	touchPad, err := pad.NewMultiTouchWriter(2)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create trackpad singletouch writer")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the keyboard, error")
	}

	return &ClamshellActionHandler{
		tconn:    tconn,
		ui:       uiauto.New(tconn).WithPollOpts(defaultPollOpt),
		kb:       kb,
		pad:      pad,
		touchPad: touchPad,
	}, nil
}

// Close releases the underlying resouses.
func (cl *ClamshellActionHandler) Close() {
	cl.kb.Close()
	cl.pad.Close()
	cl.touchPad.Close()
}

// LaunchChrome launches the Chrome browser.
func (cl *ClamshellActionHandler) LaunchChrome(ctx context.Context) (time.Time, error) {
	return LaunchAppFromShelf(ctx, cl.tconn, "Chrome", "Chromium")
}

func (cl *ClamshellActionHandler) clickChromeOnShelf(ctx context.Context) (time.Time, error) {
	return LaunchAppFromShelf(ctx, cl.tconn, "Chrome", "Chromium")
}

func (cl *ClamshellActionHandler) clickOpenedAppOnShelf(ctx context.Context, appName string) (time.Time, error) {
	icon, err := openedAppIconFinder(ctx, cl.tconn, appName)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to find window with name")
	}
	startTime := time.Now()
	if err := cl.ui.LeftClick(icon)(ctx); err != nil {
		return time.Time{}, errors.Wrap(err, "failed to tap app icon on hotseat")
	}

	return startTime, nil
}

// NewChromeTab creates a new tab of Google Chrome.
// newWindow decide this new tab should open in current Chrome window or open in new Chrome window.
func (cl *ClamshellActionHandler) NewChromeTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
	if newWindow {
		return cr.NewConn(ctx, url, cdputil.WithNewWindow())
	}

	if err := cl.kb.Accel(ctx, "Ctrl+T"); err != nil {
		return nil, errors.Wrap(err, "failed to hit Ctrl-T")
	}

	c, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find new tab")
	}
	if err := c.Navigate(ctx, url); err != nil {
		if err := c.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close target: ", err)
		}
		if err := c.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close the connection: ", err)
		}
		return nil, errors.Wrapf(err, "failed to navigate to %s, error: %v", url, err)
	}

	return c, nil
}

// SwitchWindow switches to the next window by key event.
func (cl *ClamshellActionHandler) SwitchWindow(ctx context.Context) error {
	return cl.kb.Accel(ctx, "Alt+Tab")
}

// SwitchToAppWindow switches to the window of the given app.
// If the APP has multiple windows, it will switch to the first window.
func (cl *ClamshellActionHandler) SwitchToAppWindow(ctx context.Context, appName string) error {
	return cl.SwitchToAppWindowByIndex(ctx, appName, 0)
}

// SwitchToAppWindowByIndex switches to the specific window identified by the window index of the given APP.
// It is used when the APP has multiple windows.
func (cl *ClamshellActionHandler) SwitchToAppWindowByIndex(ctx context.Context, appName string, targetIdx int) error {
	testing.ContextLogf(ctx, "Switching to app window, by index (%d)", targetIdx)
	// The first one (which is the name of the app) should be skipped.
	menuItem := nodewith.ClassName("MenuItemView").Nth(targetIdx + 1)
	return cl.switchToWindowThroughShelf(ctx, appName, menuItem)
}

// SwitchToAppWindowByName switches to the specific window identified by the window name of the given APP.
// It is used when the APP has multiple windows.
func (cl *ClamshellActionHandler) SwitchToAppWindowByName(ctx context.Context, appName, targetName string) error {
	testing.ContextLogf(ctx, "Switching to app %s window, by name (%s)", appName, targetName)
	menuItem := nodewith.ClassName("MenuItemView").NameContaining(appName)
	return cl.switchToWindowThroughShelf(ctx, appName, menuItem)
}

// SwitchToLRUWindow switches the window to LRU (Least Recently Used) one.
// opt specifies the way of switching.
func (cl *ClamshellActionHandler) SwitchToLRUWindow(ctx context.Context, opt SwitchWindowOption) error {
	msg := ""
	switch opt {
	case SwitchWindowThroughOverview:
		testing.ContextLog(ctx, "Switching to LRU window, by overview")
		return cl.switchToLRUWindowThroughOverview(ctx)
	case SwitchWindowThroughKeyEvent:
		testing.ContextLog(ctx, "Switching to app window, by key event")
		ws, err := ash.GetAllWindows(ctx, cl.tconn)
		if err != nil {
			return errors.Wrap(err, "failed get current windows")
		}
		return cl.switchToLRUWindowThroughKeyEvent(ctx, len(ws))
	case SwitchWindowThroughShelf:
		msg = "switch to LRU window through shelf not support for ClamshellActionHandler, please use SwitchWindowThroughOverview or SwitchWindowThroughKeyEvent instead"
	}
	return errors.New(msg)
}

// switchToWindowThroughShelf switch current focus window to another through shelf.
func (cl *ClamshellActionHandler) switchToWindowThroughShelf(ctx context.Context, appName string, menuItemFinder *nodewith.Finder) error {
	if strings.Contains(appName, "Chrome") || strings.Contains(appName, "Chromium") {
		if _, err := cl.clickChromeOnShelf(ctx); err != nil {
			return errors.Wrap(err, "failed to click Chrome app icon on shelf")
		}
	} else {
		if _, err := cl.clickOpenedAppOnShelf(ctx, appName); err != nil {
			return errors.Wrapf(err, "failed to click [%s] app icon on shelf", appName)
		}
	}

	if err := cl.ui.LeftClick(menuItemFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to tap app icon on shelf")
	}

	return nil
}

// switchToLRUWindowThroughKeyEvent switches current focus window to least recently used one through alt+tab.
func (cl *ClamshellActionHandler) switchToLRUWindowThroughKeyEvent(ctx context.Context, numWindows int) error {
	// No need to switch if there is only one window exist.
	if numWindows <= 1 {
		return nil
	}

	shortPause := func(ctx context.Context) error { return testing.Sleep(ctx, 200*time.Millisecond) }

	actions := []action.Action{cl.kb.AccelPressAction("Alt")}
	for i := 1; i < numWindows; i++ {
		actions = append(actions,
			shortPause,
			cl.kb.AccelPressAction("Tab"),
			shortPause,
			cl.kb.AccelReleaseAction("Tab"),
		)
	}
	actions = append(actions, cl.kb.AccelReleaseAction("Alt"))

	return action.Combine("Alt-Tab", actions...)(ctx)
}

// switchToLRUWindowThroughOverview switches the window to least recently used one through overview.
func (cl *ClamshellActionHandler) switchToLRUWindowThroughOverview(ctx context.Context) error {
	// Ensure overview is shown.
	if err := ash.SetOverviewModeAndWait(ctx, cl.tconn, true); err != nil {
		return errors.Wrap(err, "failed to show overview")
	}
	// Ensure overview is hidden even if an error occur.
	defer ash.SetOverviewModeAndWait(ctx, cl.tconn, false)

	targetWindowFinder, err := overviewLRUWindowFinder(ctx, cl.ui)
	if err != nil {
		return errors.Wrap(err, "failed to get LRU window finder")
	}

	return uiauto.Combine("Click item on overview",
		cl.ui.LeftClick(targetWindowFinder),
		cl.ui.WaitUntilGone(targetWindowFinder),
	)(ctx)
}

// SwitchChromeTab switches to the next Chrome tab.
func (cl *ClamshellActionHandler) SwitchChromeTab(ctx context.Context) error {
	testing.ContextLog(ctx, "Switching Chrome tab by key event Ctrl+Tab")
	return cl.kb.Accel(ctx, "Ctrl+Tab")
}

// SwitchToChromeTabByIndex switches the Chrome tab from one to another through UI operation.
func (cl *ClamshellActionHandler) SwitchToChromeTabByIndex(ctx context.Context, tabIdxDest int) error {
	testing.ContextLogf(ctx, "Switching to chrome tab, by index (%d)", tabIdxDest)
	tabFinder := nodewith.Role(role.Tab).ClassName("Tab").Nth(tabIdxDest)
	return cl.switchChromeTab(ctx, tabFinder)
}

// SwitchToChromeTabByName switches the Chrome tab from one to another through UI operation.
func (cl *ClamshellActionHandler) SwitchToChromeTabByName(ctx context.Context, tabNameDest string) error {
	testing.ContextLogf(ctx, "Switching Chrome tab, by name (%s)", tabNameDest)
	tabFinder := nodewith.NameContaining(tabNameDest).First().Role(role.Tab)
	return cl.switchChromeTab(ctx, tabFinder)
}

// switchChromeTab switches the Chrome tab from one to another in the current active chrome window.
func (cl *ClamshellActionHandler) switchChromeTab(ctx context.Context, tabFinder *nodewith.Finder) error {
	w, err := ash.FindWindow(ctx, cl.tconn, func(w *ash.Window) bool {
		return w.IsActive && w.IsFrameVisible
	})
	if err != nil {
		return errors.Wrap(err, "failed to get current active windows")
	}

	testing.ContextLog(ctx, "Current chrome window title: ", w.Title)
	windowNode := nodewith.Name(w.Title).Role(role.Window).ClassName("BrowserFrame")
	infos, err := cl.ui.NodesInfo(ctx, windowNode)
	if len(infos) == 0 {
		return errors.Errorf("cannot find a chrome window with title %q", w.Title)
	} else if len(infos) > 1 {
		// There are more than one chrome windows with the same name.
		// Use location to determine the active one.
		found := false
		for i, info := range infos {
			if w.BoundsInRoot.Contains(info.Location) {
				windowNode = windowNode.Nth(i)
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("cannot find an active chrome window with title %q", w.Title)
		}
	}

	tabList := nodewith.ClassName("TabStripRegionView").Role(role.TabList).Ancestor(windowNode)
	tabFinder = tabFinder.Ancestor(tabList)
	// Click the target to switch to that tab.
	return cl.ui.LeftClick(tabFinder)(ctx)
}

// RefreshChromePage refresh a web page (current focus page).
func (cl *ClamshellActionHandler) RefreshChromePage(ctx context.Context) error {
	return cl.kb.Accel(ctx, "refresh")
}

// ScrollChromePage generate the scroll action.
func (cl *ClamshellActionHandler) ScrollChromePage(ctx context.Context) ([]func(ctx context.Context, conn *chrome.Conn) error, error) {
	var (
		x      = cl.pad.Width() / 2
		ystart = cl.pad.Height() / 4
		yend   = cl.pad.Height() / 4 * 3
		d      = cl.pad.Width() / 8 // x-axis distance between two fingers.
	)

	// Move the mouse cursor to center of the page so the scrolling (by swipe) will be effected on the web page.
	// If Chrome (the browser) has been resize, then the center of screen is not guarantee to be center of window,
	// especially when there are multiple windows opened.
	prepare := func(ctx context.Context, conn *chrome.Conn) error {
		if err := cl.mouseMoveToCenterOfWindow(ctx, conn); err != nil {
			return errors.Wrap(err, "failed to prepare DoubleSwipe")
		}
		return nil
	}

	// Swipe the page down.
	doubleSwipeDown := func(ctx context.Context, conn *chrome.Conn) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, ystart, x, yend, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe down")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	doubleSwipeUp := func(ctx context.Context, conn *chrome.Conn) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, yend, x, ystart, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe up")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context, conn *chrome.Conn) error{
		prepare,
		doubleSwipeDown,
		doubleSwipeUp,
		doubleSwipeUp,
	}, nil
}

// MinimizeAllWindow minimizes all window.
func (cl *ClamshellActionHandler) MinimizeAllWindow(ctx context.Context) error {
	// Count the number of targets to minimize.
	ws, err := ash.GetAllWindows(ctx, cl.tconn)
	if err != nil {
		return errors.Wrap(err, "failed get current windows")
	}
	total := len(ws)
	testing.ContextLogf(ctx, "Found %d windows should be minimized", total)

	// Only active and frame-visible ones can be minimize by UI operation.
	// Scan until all targets are minimized.
	ui := uiauto.New(cl.tconn)
	for minimized := 0; minimized < total; {
		for _, w := range ws {
			// only active and frameVisible one is the target to minimize.
			if !w.IsActive || !w.IsFrameVisible {
				continue
			}

			// Find the button under window and click it.
			windowNode := nodewith.Name(w.Title).Role(role.Window).First()
			minimizeBtn := nodewith.Name("Minimize").Role(role.Button).ClassName("FrameCaptionButton").Ancestor(windowNode)
			if err := uiauto.Combine("find minimize button under window and click it",
				ui.WaitUntilExists(windowNode),
				ui.WaitUntilExists(minimizeBtn),
				ui.LeftClick(minimizeBtn),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to minimize window")
			}

			minimized++
			testing.ContextLogf(ctx, "Window: %q is minimized", w.Title)
			break
		}
		// Get windows again since window state changed.
		ws, err = ash.GetAllWindows(ctx, cl.tconn)
		if err != nil {
			return errors.Wrap(err, "failed get current windows")
		}
	}

	return nil
}

// mouseMoveToCenterOfWindow moves the mouse to the center of chrome window.
func (cl *ClamshellActionHandler) mouseMoveToCenterOfWindow(ctx context.Context, conn *chrome.Conn) error {
	var title string
	if err := conn.Eval(ctx, "document.title", &title); err != nil {
		return errors.Wrap(err, "failed to get current tab's title")
	}

	return cl.ui.MouseMoveTo(nodewith.Name(title).Role(role.Window).First(), 0)(ctx)
}

// openedAppIconFinder finds the opened app icon's finder from hotseat or shelf.
func openedAppIconFinder(ctx context.Context, tconn *chrome.TestConn, name string) (*nodewith.Finder, error) {
	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get hotseat items")
	}

	for _, item := range items {
		if strings.Contains(item.Title, name) || strings.Contains(name, item.Title) {
			if item.Status == ash.ShelfItemClosed {
				return nil, errors.Wrap(err, "target app is not opened")
			}
			return nodewith.ClassName("ash/ShelfAppButton").NameContaining(item.Title), nil
		}
	}

	return nil, errors.Wrapf(err, "target icon [%s] not found", name)
}

// overviewLRUWindowFinder finds the LRU item (which is the bottom right one) from overview.
func overviewLRUWindowFinder(ctx context.Context, ui *uiauto.Context) (*nodewith.Finder, error) {
	windowsFinder := nodewith.ClassName("OverviewItemView")
	windowsInfo, err := ui.NodesInfo(ctx, windowsFinder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain the overview window info")
	}

	// Find the LRU window, which is the bottom-right one.
	target := coords.NewPoint(0, 0)
	idxWindow := -1
	found := false
	for i, info := range windowsInfo {
		if info.Location.CenterPoint().Y < target.Y {
			continue
		}
		if info.Location.CenterPoint().X <= target.X {
			continue
		}

		target.X = info.Location.CenterPoint().X
		target.Y = info.Location.CenterPoint().Y
		idxWindow = i
		found = true
	}

	if !found {
		return nil, errors.Wrap(err, "failed to find the LRU window on overview")
	}

	return windowsFinder.Nth(idxWindow), nil
}
