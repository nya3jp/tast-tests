// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

var defaultPollOptions = testing.PollOptions{Timeout: time.Minute, Interval: time.Second}

// UIActionHandler defines UI actions performed either on a tablet or on a clamshell UI.
type UIActionHandler interface {
	// LaunchChrome launches the Chrome browser.
	LaunchChrome(ctx context.Context) (time.Time, error)
	// NewChromeTab creates a new tab of Google Chrome.
	NewChromeTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error)
	// SwitchWindow switches the Chrome tab from one to another.
	SwitchWindow(ctx context.Context, idxWindow, numWindows int) error
	// SwitchChromeTab switches the Chrome tab from one to another.
	SwitchChromeTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error
	// ScrollChromePage generate the scroll actions.
	ScrollChromePage() []func(ctx context.Context, conn *chrome.Conn) error
	// ChromePageRefresh refresh a web page (current focus page).
	ChromePageRefresh(ctx context.Context) error
	// Close releases the underlying resouses.
	Close()
}

// TabletActionHandler defines the action on tablet devices.
type TabletActionHandler struct {
	tconn *chrome.TestConn
	tc    *pointer.TouchController
	ui    *uiauto.Context
}

// NewTabletActionHandler returns the action handler which is responsible for handling UI actions on tablet.
func NewTabletActionHandler(ctx context.Context, tconn *chrome.TestConn) (*TabletActionHandler, error) {
	tc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the touch controller")
	}
	return &TabletActionHandler{
		tconn: tconn,
		tc:    tc,
		ui:    uiauto.New(tconn).WithPollOpts(defaultPollOptions),
	}, nil
}

// Close releases the underlying resouses.
func (t *TabletActionHandler) Close() {
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
	return t.stableTouch(ctx, toggle)
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
	if err := t.stableTouch(ctx, btn); err != nil {
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

// SwitchWindow switches the Chrome tab from one to another.
func (t *TabletActionHandler) SwitchWindow(ctx context.Context, idxWindow, numWindows int) error {
	// No need to switch if there is only one window exist.
	if numWindows <= 1 {
		return nil
	}

	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, t.tconn, t.tc.EventWriter(), t.tc.TouchCoordConverter()); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	// Click Google Chrome on hotseat.
	if _, err := t.clickChromeOnHotseat(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click new app button on hotseat")
	}
	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, t.tconn, ash.ShelfExtended); err != nil {
		return errors.Wrap(err, "failed to wait for hotseat animating to ideal")
	}

	menuItem := nodewith.ClassName("MenuItemView")

	// The first one (which is "Google Chrome") should be skipped.
	return t.stableTouch(ctx, menuItem.Nth(idxWindow+1))
}

// SwitchChromeTab switches the Chrome tab from one to another.
// with WebUITabStrip, there would be only one window at a time.
func (t *TabletActionHandler) SwitchChromeTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error {
	if tabTotalNum <= tabIdxSrc || tabTotalNum <= tabIdxDest {
		return errors.New("invalid parameters for switch tab")
	}
	// No need to switch if there is only one tab exist.
	if tabTotalNum <= 1 {
		return errors.New("only one tab exist, nothing to switch")
	}

	// Open tab list.
	if err := t.showTabList(ctx); err != nil {
		return errors.Wrap(err, "failed to open the tab list")
	}

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
	if len(tabs) < tabTotalNum {
		return errors.Errorf("tab num %d is different with expected number %d", len(tabs), tabTotalNum)
	}

	var (
		swipeDistance    input.TouchCoord
		onscreenTabWidth int
	)
	succ := false
	tcc := t.tc.TouchCoordConverter()
	// Find two adjacent items which are both fully in-screen to calculate the swipe distance.
	for i := 0; i < len(tabs)-1; i++ {
		onscreen1 := !tabs[i].State[state.Offscreen]
		onscreen2 := !tabs[i+1].State[state.Offscreen]
		width1 := tabs[i].Location.Width
		width2 := tabs[i+1].Location.Width
		if onscreen1 && onscreen2 && width1 == width2 {
			x0, _ := tcc.ConvertLocation(tabs[i].Location.CenterPoint())
			x1, _ := tcc.ConvertLocation(tabs[i+1].Location.CenterPoint())
			swipeDistance = x1 - x0
			onscreenTabWidth = width1
			succ = true
			break
		}
	}
	if !succ {
		return errors.Wrap(err, "failed to find two adjacency tab items within screen")
	}

	tabsNew := tabs
	// Check if swipe is required to show the target tab.
	if tabs[tabIdxDest].State[state.Offscreen] || tabs[tabIdxDest].Location.Width < onscreenTabWidth {
		swipeDirection := 1 // The direction of swipe. Default is right swipe.
		if tabIdxDest < tabIdxSrc {
			// Left swipe.
			swipeDirection = -1
		}

		var (
			swipeTimes = int(math.Abs(float64(tabIdxDest - tabIdxSrc)))
			ptSrc      = tcLocation.CenterPoint()
			x0, y0     = tcc.ConvertLocation(ptSrc)
			x1, y1     = x0 + swipeDistance*input.TouchCoord(swipeDirection), y0
		)

		// Do scroll.
		// The total swipe distance might greater than screen size, which means the destination point might out of screen
		// needs to separate them otherwise the swipe won't work.
		stw := t.tc.EventWriter()
		for i := 0; i < swipeTimes; i++ {
			if err := stw.Swipe(ctx, x1, y1, x0, y0, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to Swipe down")
			}
			if err := stw.End(); err != nil {
				return errors.Wrap(err, "failed to end a touch")
			}
		}

		// Get the container location again after scroll.
		tcLocation, err = t.ui.Location(ctx, tcFinder)
		if err != nil {
			return errors.Wrap(err, "failed to wait for tab list container to be stable")
		}
		testing.ContextLog(ctx, "Scroll complete, ready for tab switch")
		// Get tab items' info again since the position has changed after scroll.
		if tabsNew, err = t.ui.NodesInfo(ctx, tbFinder); err != nil {
			return errors.Wrap(err, "failed to find tab items in current window after scroll")
		}
		if len(tabsNew) < tabTotalNum {
			return errors.Errorf("tab num %d is different with expected number %d after scroll", len(tabsNew), tabTotalNum)
		}
	}

	x, y := t.tc.TouchCoordConverter().ConvertLocation(tabsNew[tabIdxDest].Location.CenterPoint())
	return t.touch(ctx, x, y)
}

// ChromePageRefresh refresh a web page (current focus page).
func (t *TabletActionHandler) ChromePageRefresh(ctx context.Context) error {
	btn := nodewith.Name("Reload").Role(role.Button).ClassName("ReloadButton").First()
	return t.stableTouch(ctx, btn)
}

// ScrollChromePage generate the scroll action.
func (t *TabletActionHandler) ScrollChromePage() []func(ctx context.Context, conn *chrome.Conn) error {
	touchScreen := t.tc.Touchscreen()
	var (
		x      = touchScreen.Width() / 2
		ystart = touchScreen.Height() / 4 * 3 // 75% of screen height
		yend   = touchScreen.Height() / 4     // 25% of screen height
	)

	// Swipe the page down.
	stw := t.tc.EventWriter()
	swipeDown := func(ctx context.Context, conn *chrome.Conn) error {
		if err := stw.Swipe(ctx, x, ystart, x, yend, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	swipeUp := func(ctx context.Context, conn *chrome.Conn) error {
		if err := stw.Swipe(ctx, x, yend, x, ystart, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := stw.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context, conn *chrome.Conn) error{
		swipeDown,
		swipeUp,
		swipeUp,
	}
}

// stableTouch presses the given UI node after its location is stable.
func (t *TabletActionHandler) stableTouch(ctx context.Context, finder *nodewith.Finder) error {
	l, err := t.ui.Location(ctx, finder)
	if err != nil {
		return errors.Wrap(err, "failed to get location")
	}

	x, y := t.tc.TouchCoordConverter().ConvertLocation(l.CenterPoint())
	return t.touch(ctx, x, y)
}

// touch executes a touch event on input coordinate.
func (t *TabletActionHandler) touch(ctx context.Context, x, y input.TouchCoord) error {
	if err := t.tc.EventWriter().Move(x, y); err != nil {
		return err
	}
	return t.tc.EventWriter().End()
}

// ClamshellActionHandler define the action on clamshell devices.
type ClamshellActionHandler struct {
	tconn    *chrome.TestConn
	kb       *input.KeyboardEventWriter
	pad      *input.TrackpadEventWriter
	touchPad *input.TouchEventWriter
	mc       *pointer.MouseController
	ui       *uiauto.Context
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
		kb:       kb,
		pad:      pad,
		touchPad: touchPad,
		mc:       pointer.NewMouseController(tconn),
		ui:       uiauto.New(tconn).WithPollOpts(defaultPollOptions),
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

// SwitchWindow switches the Chrome tab from one to another.
func (cl *ClamshellActionHandler) SwitchWindow(ctx context.Context, idxWindow, numWindows int) error {
	// No need to switch if there is only one window exist.
	if numWindows <= 1 {
		return nil
	}

	if err := cl.kb.AccelPress(ctx, "Alt"); err != nil {
		return errors.Wrap(err, "failed to execute key event")
	}
	for i := 1; i < numWindows; i++ {
		if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		if err := cl.kb.AccelPress(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
		if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		if err := cl.kb.AccelRelease(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
	}

	return cl.kb.AccelRelease(ctx, "Alt")
}

// SwitchChromeTab switches the Chrome tab from one to another.
func (cl *ClamshellActionHandler) SwitchChromeTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error {
	return cl.kb.Accel(ctx, "Ctrl+Tab")
}

// ChromePageRefresh refresh a web page (current focus page).
func (cl *ClamshellActionHandler) ChromePageRefresh(ctx context.Context) error {
	return cl.kb.Accel(ctx, "refresh")
}

// ScrollChromePage generate the scroll action.
func (cl *ClamshellActionHandler) ScrollChromePage() []func(ctx context.Context, conn *chrome.Conn) error {
	var (
		x      = cl.pad.Width() / 2
		ystart = cl.pad.Height() / 4
		yend   = cl.pad.Height() / 4 * 3
		d      = cl.pad.Width() / 8 // x-axis distance between two fingers
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
	}
}

// mouseMoveToCenterOfWindow moves the mouse to the center of chrome window.
func (cl *ClamshellActionHandler) mouseMoveToCenterOfWindow(ctx context.Context, conn *chrome.Conn) error {
	var title string
	if err := conn.Eval(ctx, "document.title", &title); err != nil {
		return errors.Wrap(err, "failed to get current tab's title")
	}

	window := nodewith.Name(title).Role(role.Window).First()
	l, err := cl.ui.Location(ctx, window)
	if err != nil {
		return err
	}

	return cl.mc.Move(ctx, l.CenterPoint(), l.CenterPoint(), 0)
}
