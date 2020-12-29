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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Use 1 minute timeout value because it may take longer to wait for UI nodes to be stable,
// especially for some low end DUTs.
var defaultPollOpts = testing.PollOptions{Timeout: time.Minute, Interval: time.Second}

// SwitchWindowOption defines types of operations of switching window.
type SwitchWindowOption int

const (
	// SwitchWindowThroughHotseat specifies switch window through hotseat.
	SwitchWindowThroughHotseat SwitchWindowOption = iota
	// SwitchWindowThroughOverview specifies switch window through overview.
	SwitchWindowThroughOverview
	// SwitchWindowThroughKeyEvent specifies switch window through key event.
	SwitchWindowThroughKeyEvent
	// SwitchWindowThroughShelf specifies switch window through shelf.
	SwitchWindowThroughShelf = SwitchWindowThroughHotseat
)

// UIActionHandler defines UI actions performed either on a tablet or on a clamshell UI.
type UIActionHandler interface {
	// Close releases the underlying resouses.
	// Tests should always defer calls to this once the UIActionHandler instance been created.
	Close()

	// Click returns a function that clicks or taps the node found by input finder.
	Click(finder *nodewith.Finder) action.Action

	// ClickUntil returns a function that repeatedly left clicks the node until the condition returns no error.
	// It will try to click the node once before it checks the condition.
	ClickUntil(finder *nodewith.Finder, condition func(context.Context) error) action.Action

	// LaunchChrome launches the Chrome browser.
	LaunchChrome(ctx context.Context) (time.Time, error)

	// NewChromeTab creates a new tab of Google Chrome.
	NewChromeTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error)

	// SwitchWindow returns a function which switches to the next window by key event.
	SwitchWindow() action.Action

	// SwitchToLRUWindow returns a function which switches the window to LRU (Least Recently Used) one.
	// opt specifies the way of switching.
	SwitchToLRUWindow(opt SwitchWindowOption) action.Action

	// SwitchToAppWindow returns a function which switches to the window of the given app.
	// If the APP has multiple windows, it will switch to the first window.
	SwitchToAppWindow(appName string) action.Action

	// SwitchToAppWindowByIndex returns a function which switches to
	// the specific window identified by the window index of the given APP.
	// It is used when the APP has multiple windows.
	SwitchToAppWindowByIndex(appName string, targetIdx int) action.Action

	// SwitchToAppWindowByName returns a function which switches to
	// the specific window identified by the window name of the given APP.
	// It is used when the APP has multiple windows.
	SwitchToAppWindowByName(appName, targetName string) action.Action

	// SwitchToNextChromeTab returns a function which switches to the next Chrome tab by key event.
	SwitchToNextChromeTab() action.Action

	// SwitchToChromeTabByIndex returns a function which switches to
	// the tab identified by the tab index in the current chrome window.
	SwitchToChromeTabByIndex(tabIdxDest int) action.Action

	// SwitchToChromeTabByName returns a function which switches the Chrome tab to
	// the one with the given name through UI operation.
	// The tab name must exact match.
	// If multiple tabs with same name, it goes to the first one.
	SwitchToChromeTabByName(tabNameDest string) action.Action

	// ScrollChromePage generate the scroll actions.
	ScrollChromePage(ctx context.Context) ([]action.Action, error)

	// MinimizeAllWindow returns a function which minimizes all window.
	MinimizeAllWindow() action.Action
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
	var (
		succ = false
		err  error
		tc   *touch.Context
		tcc  *input.TouchCoordConverter
		kb   *input.KeyboardEventWriter
		tew  *input.TouchscreenEventWriter
		stew *input.SingleTouchEventWriter
	)

	defer func() {
		if succ {
			return
		}
		if tc != nil {
			if err := tc.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close touch context")
			}
		}
		if kb != nil {
			if err := kb.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close keyboard event writer")
			}
		}
		if tew != nil {
			if err := tew.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close touchscreen event writer")
			}
		}
		if stew != nil {
			stew.Close()
		}
	}()

	if tc, err = touch.New(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to create the touch context instance")
	}
	if kb, err = input.Keyboard(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to create the keyboard, error")
	}
	// Get touch controller for tablet.
	if tew, tcc, err = touch.NewTouchscreenAndConverter(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to create the touchscreen and converter")
	}
	if stew, err = tew.NewSingleTouchWriter(); err != nil {
		return nil, errors.Wrap(err, "failed to create the single touch writer")
	}

	succ = true
	return &TabletActionHandler{
		tconn: tconn,
		kb:    kb,
		ui:    uiauto.New(tconn).WithPollOpts(defaultPollOpts),
		tc:    tc.WithPollOpts(defaultPollOpts),
		tew:   tew,
		tcc:   tcc,
		stew:  stew,
	}, nil
}

// Close releases the underlying resouses.
// Tests should always defer calls to this once the UIActionHandler instance been created.
func (t *TabletActionHandler) Close() {
	t.kb.Close()
	t.stew.Close()
	t.tc.Close()
}

// Click returns a function that taps the node found by input finder on tablet.
func (t *TabletActionHandler) Click(finder *nodewith.Finder) action.Action {
	return t.tc.Tap(finder)
}

// ClickUntil returns a function that repeatedly left clicks the node until the condition returns no error.
// It will try to click the node once before it checks the condition.
func (t *TabletActionHandler) ClickUntil(finder *nodewith.Finder, condition func(context.Context) error) action.Action {
	return func(ctx context.Context) error {
		if err := uiauto.Combine("initially click the node",
			t.tc.Tap(finder),
			t.ui.Sleep(defaultPollOpts.Interval),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to initially click the node")
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := condition(ctx); err != nil {
				loc, err := t.ui.ImmediateLocation(ctx, finder)
				if err != nil {
					return err
				}
				if err := t.tc.TapAt(loc.CenterPoint())(ctx); err != nil {
					return errors.Wrap(err, "failed to click the node")
				}
				return errors.Wrap(err, "click may not have been received yet")
			}
			return nil
		}, &defaultPollOpts)
	}
}

// LaunchChrome launches the Chrome browser.
func (t *TabletActionHandler) LaunchChrome(ctx context.Context) (time.Time, error) {
	return t.clickChromeOnHotseat(ctx)
}

func (t *TabletActionHandler) clickChromeOnHotseat(ctx context.Context) (time.Time, error) {
	return LaunchAppFromHotseat(ctx, t.tconn, "Chrome", "Chromium")
}

// showTabList shows the tab list by clicking a button on the Chrome tool bar.
func (t *TabletActionHandler) showTabList() action.Action {
	// There may be multiple browser windows under tablet mode, with one active and others invisible.
	// The UI layout of different windows are the same and with the same coordinates. So tap the first
	// found button will trigger the tap on the same button of the active window.
	toggle := nodewith.NameContaining("press to toggle tab strip").ClassName("WebUITabCounterButton").Role(role.Button).First()
	tcFinder := nodewith.Role(role.TabList).Ancestor(nodewith.Role(role.RootWebArea).Name("Tab list"))
	return uiauto.Combine("show tab list",
		t.tc.Tap(toggle),
		t.ui.WaitUntilExists(tcFinder),
	)
}

// NewChromeTab creates a new tab of Google Chrome.
// newWindow indicates whether this new tab should open in current Chrome window or in new Chrome window.
func (t *TabletActionHandler) NewChromeTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if newWindow {
		return cr.NewConn(ctx, url, cdputil.WithNewWindow())
	}

	// There may be multiple browser windows under tablet mode, with one active and others invisible.
	// The UI layout of different windows are the same and with the same coordinates. So tap the first
	// found button will trigger the tap on the same button of the active window.
	newTabFinder := nodewith.Name("New tab").Role(role.Button).ClassName("ToolbarButton").First()
	if err := t.tc.Tap(newTabFinder)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to tap new tab button")
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

// SwitchWindow returns a function which switches to the next window by key event.
func (t *TabletActionHandler) SwitchWindow() action.Action {
	return t.kb.AccelAction("Alt+Tab")
}

// SwitchToAppWindow returns a function which switches to the window of the given app.
// If the APP has multiple windows, it will switch to the first window.
func (t *TabletActionHandler) SwitchToAppWindow(appName string) action.Action {
	return t.SwitchToAppWindowByIndex(appName, 0)
}

// SwitchToAppWindowByIndex returns a function which switches to
// the specific window identified by the window index of the given APP.
// It is used when the APP has multiple windows.
func (t *TabletActionHandler) SwitchToAppWindowByIndex(appName string, targetIdx int) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching to app window, by index (%d)", targetIdx)
		// The first one (which is the name of the app) should be skipped.
		menuItem := nodewith.ClassName("MenuItemView").Nth(targetIdx + 1)
		return t.switchToWindowThroughHotseat(ctx, appName, menuItem)
	}
}

// SwitchToAppWindowByName returns a function which switches to
// the specific window identified by the window name of the given APP.
// It is used when the APP has multiple windows.
func (t *TabletActionHandler) SwitchToAppWindowByName(appName, targetName string) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching to app %s window, by name (%s)", appName, targetName)
		menuItem := nodewith.ClassName("MenuItemView").NameContaining(targetName).First()
		return t.switchToWindowThroughHotseat(ctx, appName, menuItem)
	}
}

// switchToWindowThroughHotseat switch current focus window to another through hotseat.
func (t *TabletActionHandler) switchToWindowThroughHotseat(ctx context.Context, appName string, menuItemFinder *nodewith.Finder) error {
	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, t.tconn, t.stew, t.tcc); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	if strings.Contains(appName, "Chrome") || strings.Contains(appName, "Chromium") {
		if _, err := t.clickChromeOnHotseat(ctx); err != nil {
			return errors.Wrap(err, "failed to click Chrome app icon on hotseat")
		}
	} else {
		icon, _, err := openedAppIconFinder(ctx, t.tconn, appName)
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

// SwitchToLRUWindow returns a function which switches to the LRU (Least Recently Used) window.
// opt specifies the way of switching.
func (t *TabletActionHandler) SwitchToLRUWindow(opt SwitchWindowOption) action.Action {
	return func(ctx context.Context) error {
		switch opt {
		case SwitchWindowThroughOverview:
			testing.ContextLog(ctx, "Switching to LRU window, by overview")
			return t.switchToLRUWindowByOverview(ctx)
		default:
			return errors.Errorf("switch to LRU window option %d is not supported on tablet", opt)
		}
	}
}

// switchToLRUWindowByOverview switches the window to least recently used one through overview.
func (t *TabletActionHandler) switchToLRUWindowByOverview(ctx context.Context) error {
	// Ensure overview is shown.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.DragToShowOverview(ctx, t.tew, t.stew, t.tconn)
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to show overview")
	}
	// Hide overview mode before the function returns.
	// If the LUR window has been successfully clicked, overview window should have been hidden
	// already. But it's still okay to call this function.
	defer ash.SetOverviewModeAndWait(ctx, t.tconn, false)

	targetWindowFinder, err := overviewLRUWindowFinder(ctx, t.ui)
	if err != nil {
		return errors.Wrap(err, "failed to get LRU window finder")
	}

	return uiauto.Combine("click item on overview",
		t.tc.Tap(targetWindowFinder),
		t.ui.WaitUntilGone(targetWindowFinder),
	)(ctx)
}

// SwitchToNextChromeTab returns a function which switches to the next Chrome tab by key event.
func (t *TabletActionHandler) SwitchToNextChromeTab() action.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switching Chrome tab by key event Ctrl+Tab")
		return t.kb.Accel(ctx, "Ctrl+Tab")
	}
}

// SwitchToChromeTabByIndex returns a function which switches to
// the tab identified by the tab index in the current chrome window.
func (t *TabletActionHandler) SwitchToChromeTabByIndex(tabIdxDest int) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching Chrome tab, by index (%d)", tabIdxDest)
		// Open tab list.
		if err := t.showTabList()(ctx); err != nil {
			return errors.Wrap(err, "failed to open the tab list")
		}
		return t.switchToChromeTabByIndex(ctx, tabIdxDest)
	}
}

// SwitchToChromeTabByName returns a function which switches the Chrome tab to
// the one with the given name through UI operation.
// The tab name must exact match.
// If multiple tabs with same name, it goes to the first one.
func (t *TabletActionHandler) SwitchToChromeTabByName(tabNameDest string) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching Chrome tab, by name (%s)", tabNameDest)
		// Open tab list.
		if err := t.showTabList()(ctx); err != nil {
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

		return t.switchToChromeTabByIndex(ctx, tabIdxDest)
	}
}

// switchToChromeTabByIndex switches the Chrome tab to another one identified by tab index in the current chrome window.
func (t *TabletActionHandler) switchToChromeTabByIndex(ctx context.Context, tabIdxDest int) error {
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

		// Wait location to become stable after scroll.
		if err := t.ui.WaitForLocation(tbFinder.Nth(tabIdxDest))(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for tab list container to be stable")
		}
		testing.ContextLog(ctx, "Scroll complete, ready for tab switch")
	}

	return t.tc.Tap(tbFinder.Nth(tabIdxDest))(ctx)
}

// ScrollChromePage generate the scroll action.
func (t *TabletActionHandler) ScrollChromePage(ctx context.Context) ([]action.Action, error) {
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
	swipeDown := func(ctx context.Context) error {
		if err := t.tc.Swipe(start, t.tc.SwipeTo(end, 500*time.Millisecond))(ctx); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		return nil
	}

	// Swipe the page up.
	swipeUp := func(ctx context.Context) error {
		if err := t.tc.Swipe(end, t.tc.SwipeTo(start, 500*time.Millisecond))(ctx); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		return nil
	}

	return []func(ctx context.Context) error{
		swipeDown,
		swipeUp,
		swipeUp,
	}, nil
}

// MinimizeAllWindow returns a function which minimizes all window.
func (t *TabletActionHandler) MinimizeAllWindow() action.Action {
	return func(ctx context.Context) error {
		if err := ash.DragToShowHomescreen(ctx, t.tew.Width(), t.tew.Height(), t.stew, t.tconn); err != nil {
			return errors.Wrap(err, "failed to show homescreen")
		}
		if err := ash.WaitForHotseatAnimatingToIdealState(ctx, t.tconn, ash.ShelfShownHomeLauncher); err != nil {
			return errors.Wrap(err, "hotseat is in an unexpected state")
		}
		testing.ContextLog(ctx, "All windows are minimized")
		return nil
	}
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
	var (
		succ     = false
		err      error
		pad      *input.TrackpadEventWriter
		touchPad *input.TouchEventWriter
		kb       *input.KeyboardEventWriter
	)

	defer func() {
		if succ {
			return
		}
		if pad != nil {
			if err := pad.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close trackpad event writer")
			}
		}
		if touchPad != nil {
			touchPad.Close()
		}
		if kb != nil {
			if err := kb.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close keyboard event writer")
			}
		}
	}()

	if pad, err = input.Trackpad(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to create trackpad event writer")
	}
	if touchPad, err = pad.NewMultiTouchWriter(2); err != nil {
		return nil, errors.Wrap(err, "failed to create trackpad singletouch writer")
	}
	if kb, err = input.Keyboard(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to create the keyboard, error")
	}

	succ = true
	return &ClamshellActionHandler{
		tconn:    tconn,
		ui:       uiauto.New(tconn).WithPollOpts(defaultPollOpts),
		kb:       kb,
		pad:      pad,
		touchPad: touchPad,
	}, nil
}

// Close releases the underlying resouses.
// Tests should always defer calls to this once the UIActionHandler instance been created.
func (cl *ClamshellActionHandler) Close() {
	cl.kb.Close()
	cl.pad.Close()
	cl.touchPad.Close()
}

// Click returns a function that does left-click on the node found by input finder on clamshell.
func (cl *ClamshellActionHandler) Click(finder *nodewith.Finder) action.Action {
	return cl.ui.LeftClick(finder)
}

// ClickUntil returns a function that repeatedly left clicks the node until the condition returns no error.
// It will try to click the node once before it checks the condition.
func (cl *ClamshellActionHandler) ClickUntil(finder *nodewith.Finder, condition func(context.Context) error) action.Action {
	return cl.ui.LeftClickUntil(finder, condition)
}

// LaunchChrome launches the Chrome browser.
func (cl *ClamshellActionHandler) LaunchChrome(ctx context.Context) (time.Time, error) {
	return LaunchAppFromShelf(ctx, cl.tconn, "Chrome", "Chromium")
}

func (cl *ClamshellActionHandler) clickOpenedAppOnShelf(ctx context.Context, appName string) (time.Time, error) {
	icon, appID, err := openedAppIconFinder(ctx, cl.tconn, appName)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "failed to find opened app icon for %q", appName)
	}

	var startTime time.Time
	waitAppAppear := func(ctx context.Context) error { return ash.WaitForApp(ctx, cl.tconn, appID, 15*time.Second) }

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		startTime = time.Now()
		return uiauto.Combine("click app icon",
			cl.ui.LeftClick(icon),
			waitAppAppear,
		)(ctx)
	}, &defaultPollOpts); err != nil {
		return time.Time{}, errors.Wrap(err, "failed to click app icon on shelf")
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

// SwitchWindow returns a function which switches to the next window by key event.
func (cl *ClamshellActionHandler) SwitchWindow() action.Action {
	return func(ctx context.Context) error {
		return cl.kb.Accel(ctx, "Alt+Tab")
	}
}

// SwitchToAppWindow returns a function which switches to the window of the given app.
// If the APP has multiple windows, it will switch to the first window.
func (cl *ClamshellActionHandler) SwitchToAppWindow(appName string) action.Action {
	return cl.SwitchToAppWindowByIndex(appName, 0)
}

// SwitchToAppWindowByIndex returns a function which switches to
// the specific window identified by the window name of the given APP.
// It is used when the APP has multiple windows.
func (cl *ClamshellActionHandler) SwitchToAppWindowByIndex(appName string, targetIdx int) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching to app window, by index (%d)", targetIdx)
		// The first one (which is the name of the app) should be skipped.
		menuItem := nodewith.ClassName("MenuItemView").Nth(targetIdx + 1)
		return cl.switchToWindowThroughShelf(ctx, appName, menuItem)
	}
}

// SwitchToAppWindowByName switches to the specific window identified by the window name of the given APP.
// It is used when the APP has multiple windows.
func (cl *ClamshellActionHandler) SwitchToAppWindowByName(appName, targetName string) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching to app %s window, by name (%s)", appName, targetName)
		menuItem := nodewith.ClassName("MenuItemView").NameContaining(appName)
		return cl.switchToWindowThroughShelf(ctx, appName, menuItem)
	}
}

// SwitchToLRUWindow returns a function which switches the window to LRU (Least Recently Used) one.
// opt specifies the way of switching.
func (cl *ClamshellActionHandler) SwitchToLRUWindow(opt SwitchWindowOption) action.Action {
	return func(ctx context.Context) error {
		switch opt {
		case SwitchWindowThroughOverview:
			testing.ContextLog(ctx, "Switching to LRU window, by overview")
			return cl.switchToLRUWindowThroughOverview(ctx)
		case SwitchWindowThroughKeyEvent:
			testing.ContextLog(ctx, "Switching to app window, by key event")
			ws, err := ash.GetAllWindows(ctx, cl.tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get all windows")
			}
			return cl.switchToLRUWindowThroughKeyEvent(ctx, len(ws))
		default:
			return errors.Errorf("switch to LRU window with option %d is not support for clamshell", opt)
		}
	}
}

// switchToWindowThroughShelf switch current focus window to another through shelf.
func (cl *ClamshellActionHandler) switchToWindowThroughShelf(ctx context.Context, appName string, menuItemFinder *nodewith.Finder) error {
	if strings.Contains(appName, "Chrome") || strings.Contains(appName, "Chromium") {
		app, err := apps.ChromeOrChromium(ctx, cl.tconn)
		if err != nil {
			return errors.Wrap(err, "failed to check Chrome browser for current build")
		}
		items, err := ash.ShelfItems(ctx, cl.tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get shelf items")
		}
		for _, item := range items {
			if item.AppID == app.ID {
				appName = item.Title
			}
		}
	}

	if _, err := cl.clickOpenedAppOnShelf(ctx, appName); err != nil {
		return errors.Wrapf(err, "failed to click [%s] app icon on shelf", appName)
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

	actions := []action.Action{cl.kb.AccelPressAction("Alt")}
	for i := 1; i < numWindows; i++ {
		actions = append(actions,
			cl.ui.Sleep(200*time.Millisecond),
			cl.kb.AccelPressAction("Tab"),
			cl.ui.Sleep(200*time.Millisecond),
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

	return cl.ui.LeftClickUntil(targetWindowFinder, cl.ui.Gone(targetWindowFinder))(ctx)
}

// SwitchToNextChromeTab returns a function which switches to the next Chrome tab by key event.
func (cl *ClamshellActionHandler) SwitchToNextChromeTab() action.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Switching Chrome tab by key event Ctrl+Tab")
		return cl.kb.Accel(ctx, "Ctrl+Tab")
	}
}

// SwitchToChromeTabByIndex returns a function which switches to
// the tab identified by the tab index in the current chrome window.
func (cl *ClamshellActionHandler) SwitchToChromeTabByIndex(tabIdxDest int) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching to chrome tab, by index (%d)", tabIdxDest)
		tabFinder := nodewith.Role(role.Tab).ClassName("Tab").Nth(tabIdxDest)
		return cl.switchChromeTab(ctx, tabFinder)
	}
}

// SwitchToChromeTabByName returns a function which switches the Chrome tab to
// the one with the given name through UI operation.
// The tab name must exact match.
// If multiple tabs with same name, it goes to the first one.
func (cl *ClamshellActionHandler) SwitchToChromeTabByName(tabNameDest string) action.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Switching Chrome tab, by name (%s)", tabNameDest)
		tabFinder := nodewith.Name(tabNameDest).Role(role.Tab).First()
		return cl.switchChromeTab(ctx, tabFinder)
	}
}

// switchChromeTab switches the Chrome tab from one to another in the current active chrome window.
func (cl *ClamshellActionHandler) switchChromeTab(ctx context.Context, tabFinder *nodewith.Finder) error {
	// findActiveChromeWindowAndClickTab finds the current active Chrome window, uses its title to match
	// the ui node, and then clicks the tab inside this node.
	// The function will fail if the window title changes during the process. So We'll using testing.Poll()
	// to run this function until it succeeds.
	findActiveChromeWindowAndClickTab := func(ctx context.Context) error {
		w, err := ash.FindWindow(ctx, cl.tconn, func(w *ash.Window) bool {
			return w.IsActive && w.IsFrameVisible
		})
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get current active window"))
		}
		if w.Name != "BrowserFrame" {
			return testing.PollBreak(errors.New("active window is not a browser"))
		}

		testing.ContextLog(ctx, "Current chrome window title: ", w.Title)
		windowNode := nodewith.Name(w.Title).Role(role.Window).ClassName(w.Name)
		infos, err := cl.ui.NodesInfo(ctx, windowNode)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get window nodes info"))
		}
		if len(infos) == 0 {
			return errors.Errorf("cannot find a chrome window with title %q", w.Title)
		}
		if len(infos) != 1 {
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
		return uiauto.Combine("find tab and click",
			// We want to quickly know whether we have found the tab before issuing a left click
			// with a longer timeout.
			// Wait for the tab to exist with a short timeout.
			cl.ui.WithTimeout(5*time.Second).WaitUntilExists(tabFinder),
			// Do left click with a longer timeout because the tab takes time to become stable on
			// some low end models.
			cl.ui.WithTimeout(15*time.Second).LeftClick(tabFinder),
		)(ctx)

	}
	return testing.Poll(ctx, findActiveChromeWindowAndClickTab, &defaultPollOpts)
}

// ScrollChromePage generate the scroll action.
func (cl *ClamshellActionHandler) ScrollChromePage(ctx context.Context) ([]action.Action, error) {
	var (
		x      = cl.pad.Width() / 2
		ystart = cl.pad.Height() / 4
		yend   = cl.pad.Height() / 4 * 3
		d      = cl.pad.Width() / 8 // x-axis distance between two fingers.
	)

	// Move the mouse cursor to center of the page so the scrolling (by swipe) will be effected on the web page.
	// If Chrome (the browser) has been resized, then the center of screen is not guarantee to be center of window,
	// especially when there are multiple windows opened.
	prepare := func(ctx context.Context) error {
		if err := cl.mouseMoveToCenterOfActiveWindow(ctx); err != nil {
			return errors.Wrap(err, "failed to prepare DoubleSwipe")
		}
		return nil
	}

	// Swipe the page down.
	doubleSwipeDown := func(ctx context.Context) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, ystart, x, yend, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe down")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	doubleSwipeUp := func(ctx context.Context) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, yend, x, ystart, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe up")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context) error{
		prepare,
		doubleSwipeDown,
		doubleSwipeUp,
		doubleSwipeUp,
	}, nil
}

// MinimizeAllWindow returns a function which minimizes all window.
func (cl *ClamshellActionHandler) MinimizeAllWindow() action.Action {
	return func(ctx context.Context) error {
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
}

// mouseMoveToCenterOfActiveWindow moves the mouse to the center of active chrome window.
func (cl *ClamshellActionHandler) mouseMoveToCenterOfActiveWindow(ctx context.Context) error {
	w, err := ash.FindWindow(ctx, cl.tconn, func(w *ash.Window) bool {
		return w.IsActive && w.IsFrameVisible
	})
	if err != nil {
		return errors.Wrap(err, "failed to get current active window")
	}
	return mouse.Move(cl.tconn, w.BoundsInRoot.CenterPoint(), 0)(ctx)
}

// openedAppIconFinder finds the opened app icon's finder from hotseat or shelf.
func openedAppIconFinder(ctx context.Context, tconn *chrome.TestConn, name string) (*nodewith.Finder, string, error) {
	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to get hotseat items")
	}

	appClosed := false
	nth := 0
	for _, item := range items {
		if item.Title == name {
			if item.Status == ash.ShelfItemClosed {
				appClosed = true
				nth++
				continue
			}
			// APP is found and not closed.
			return nodewith.ClassName("ash/ShelfAppButton").Name(item.Title).Nth(nth), item.AppID, nil
		}
	}

	// APP is found but closed.
	if appClosed {
		return nil, "", errors.Wrap(err, "target app is closed")
	}

	// APP is not found.
	return nil, "", errors.Wrapf(err, "target icon [%s] not found", name)
}

// overviewLRUWindowFinder finds the LRU item (which is the bottom right one) from overview.
func overviewLRUWindowFinder(ctx context.Context, ui *uiauto.Context) (*nodewith.Finder, error) {
	windowsFinder := nodewith.ClassName("OverviewItemView")
	windowsInfo, err := ui.NodesInfo(ctx, windowsFinder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain the overview window info")
	}
	if len(windowsInfo) == 0 {
		return nil, errors.New("there is no window under overview mode")
	}

	// Find the LRU window, which is the bottom-right one under the row-major ordering.
	// In the example below, we'll find out w8.
	//           Col 1    Col 2.  Col 3.
	// Row 1.     w1.      w2.     w3.
	// Row 2.     w4.      w5.     w6.
	// Row 3.     w7.      w8.
	x0 := -1
	y0 := -1
	var idxWindow int
	for i, info := range windowsInfo {
		x := info.Location.CenterPoint().X
		y := info.Location.CenterPoint().Y
		if y > y0 || (y == y0 && x > x0) {
			// New point is on a larger row, or on the same row and a larger column.
			x0 = x
			y0 = y
			idxWindow = i
		}
	}

	return windowsFinder.Nth(idxWindow), nil
}
