// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tabswitchcuj

import (
	"context"
	"math"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// uiActionHandler defines UI actions performed either on a tablet or on a clamshell UI.
type uiActionHandler interface {
	// launchChrome launches the Chrome browser.
	launchChrome(ctx context.Context) (time.Time, error)
	// newTab creates a new tab of Google Chrome.
	newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error)
	// switchWindow switches the Chrome tab from one to another.
	switchWindow(ctx context.Context, idxWindow, numWindows int) error
	// switchTab switches the Chrome tab from one to another.
	switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error
	// scrollPage generate the scroll actions.
	scrollPage() []func(ctx context.Context, conn *chrome.Conn) error
	// pageRefresh refresh a web page (current focus page).
	pageRefresh(ctx context.Context) error
}

// clamshellActionHandler defines the action on tablet devices.
type tabletActionHandler struct {
	tconn *chrome.TestConn
	tc    *pointer.TouchController
	pc    pointer.Controller
}

// newTabletActionHandler returns the action handler which is responsible for handling UI actions on tablet.
func newTabletActionHandler(tconn *chrome.TestConn, tc *pointer.TouchController) *tabletActionHandler {
	return &tabletActionHandler{
		tconn: tconn,
		tc:    tc,
		pc:    tc,
	}
}

// launchChrome launches the Chrome browser.
func (t *tabletActionHandler) launchChrome(ctx context.Context) (time.Time, error) {
	return t.clickChromeOnHotseat(ctx)
}

func (t *tabletActionHandler) clickChromeOnHotseat(ctx context.Context) (time.Time, error) {
	return cuj.LaunchAppFromHotseat(ctx, t.tconn, "Chrome", "Chromium")
}

// showTabList shows the tab list by clicking a button on the Chrome tool bar.
func (t *tabletActionHandler) showTabList(ctx context.Context) error {
	toggle := nodewith.NameRegex(regexp.MustCompile("toggle tab strip")).Role(role.Button).First()
	return t.tc.StableLeftClick(ctx, t.tconn, toggle, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
}

// newTab creates a new tab of Google Chrome.
// newWindow indicates whether this new tab should open in current Chrome window or in new Chrome window.
func (t *tabletActionHandler) newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if newWindow {
		return cr.NewConn(ctx, url, cdputil.WithNewWindow())
	}

	if err := t.showTabList(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open the tab list")
	}

	btn := nodewith.Name("New tab").Role(role.Button).First()
	if err := t.tc.StableLeftClick(ctx, t.tconn, btn, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
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

// switchWindow switches the Chrome tab from one to another.
func (t *tabletActionHandler) switchWindow(ctx context.Context, idxWindow, numWindows int) error {
	// No need to switch if there is only one window exist.
	if numWindows <= 1 {
		return nil
	}

	// Ensure hotseat is shown.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, t.tconn, t.tc.EventWriter(), t.tc.TouchCoordConverter()); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	// Click Google Chrome on hotseat.
	if _, err := t.clickChromeOnHotseat(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click new app button on hotseat")
	}
	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, t.tconn, ash.ShelfExtended); err != nil {
		return errors.Wrap(err, "failed to wait for hotseat animating to ideal")
	}

	ui := uiauto.New(t.tconn)
	menuItem := nodewith.ClassName("MenuItemView")

	if err := ui.WaitForLocation(menuItem.First())(ctx); err != nil {
		return errors.Wrap(err, "failed to wait menu items' location be stable")
	}

	nodes, err := ui.NodesInfo(ctx, menuItem)
	if err != nil {
		return err
	}

	// the first one (which is "Google Chrome") should be skip
	idxWindow++

	if idxWindow >= len(nodes) {
		return errors.New("windows number is not match")
	}

	if err := t.pc.Press(ctx, nodes[idxWindow].Location.CenterPoint()); err != nil {
		return errors.New("failed to execute touch press")
	}

	return t.pc.Release(ctx)
}

// switchTab switches the Chrome tab from one to another.
// with WebUITabStrip, there would be only one window at a time.
func (t *tabletActionHandler) switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error {
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
	// Target tab item could be out of screen.
	// Scroll the tab list to ensure the item is visible.
	testing.ContextLog(ctx, "Scrolling the tab list before tab switch")

	// Find all tab list.
	tabListContainers, err := ui.FindAll(ctx, t.tconn, ui.FindParams{Role: ui.RoleTypeTabList})
	if err != nil {
		return errors.Wrap(err, "failed to find tab list container")
	}
	defer tabListContainers.Release(ctx)

	var tabListContainer *ui.Node
	// Looking for the node located at top left of the screen, which is the current window.
	for _, tl := range tabListContainers {
		if tl.Location.Top == 0 && tl.Location.Left == 0 {
			tabListContainer = tl
			break
		}
	}
	if tabListContainer == nil {
		return errors.Wrap(err, "failed to find active tab list container")
	}
	// Wait for change completed before clicking.
	if err := ui.WaitForLocationChangeCompletedOnNode(ctx, t.tconn, tabListContainer); err != nil {
		return errors.Wrap(err, "failed to wait for location changes completed on tab list")
	}

	// Find tab items under tabListContainer.
	tabs, err := tabListContainer.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeTab})
	if err != nil {
		return errors.Wrap(err, "failed to find current focused tab item")
	}
	defer tabs.Release(ctx)
	if len(tabs) < tabTotalNum {
		return errors.New("error on finding tab items")
	}

	var (
		swipeDistance    input.TouchCoord
		onscreenTabWidth int
	)
	succ := false
	tcc := t.tc.TouchCoordConverter()
	// Find two adjacent items which are both fully in-screen to calculate the swipe distance.
	for i := 0; i < len(tabs)-1; i++ {
		onscreen1 := !tabs[i].State[ui.StateTypeOffscreen]
		onscreen2 := !tabs[i+1].State[ui.StateTypeOffscreen]
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
	if tabs[tabIdxDest].State[ui.StateTypeOffscreen] || tabs[tabIdxDest].Location.Width < onscreenTabWidth {
		swipeDirection := 1 // The direction of swipe. Default is right swipe.
		if tabIdxDest < tabIdxSrc {
			// Left swipe.
			swipeDirection = -1
		}

		var (
			swipeTimes = int(math.Abs(float64(tabIdxDest - tabIdxSrc)))
			ptSrc      = tabListContainer.Location.CenterPoint()
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

		if err := ui.WaitForLocationChangeCompletedOnNode(ctx, t.tconn, tabListContainer); err != nil {
			return errors.Wrap(err, "failed to wait for location changes")
		}
		testing.ContextLog(ctx, "Scroll complete, ready for tab switch")
		// Find tab items again since the position is changed after scroll.
		if tabsNew, err = tabListContainer.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeTab}); err != nil {
			return errors.Wrap(err, "failed to find current focused tab items after scroll")
		}
		defer tabsNew.Release(ctx)
	}

	if len(tabsNew) < tabTotalNum {
		return errors.Errorf("tab num %d is different with expected number %d", len(tabsNew), tabTotalNum)
	}

	if err := t.pc.Press(ctx, tabsNew[tabIdxDest].Location.CenterPoint()); err != nil {
		return errors.New("failed to execute touch press")
	}

	return t.pc.Release(ctx)
}

// pageRefresh refresh a web page (current focus page).
func (t *tabletActionHandler) pageRefresh(ctx context.Context) error {
	btn := nodewith.Name("Reload").Role(role.Button).ClassName("ReloadButton").First()
	return t.tc.StableLeftClick(ctx, t.tconn, btn, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second})
}

// scrollPage generate the scroll action.
func (t *tabletActionHandler) scrollPage() []func(ctx context.Context, conn *chrome.Conn) error {
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

// clamshellActionHandler define the action on clamshell devices.
type clamshellActionHandler struct {
	tconn    *chrome.TestConn
	kb       *input.KeyboardEventWriter
	pad      *input.TrackpadEventWriter
	touchPad *input.TouchEventWriter
	pc       pointer.Controller
}

// newClamshellActionHandler returns the action handler which is responsible for handling UI actions on clamshell.
func newClamshellActionHandler(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, pad *input.TrackpadEventWriter, touchPad *input.TouchEventWriter) *clamshellActionHandler {
	return &clamshellActionHandler{
		tconn:    tconn,
		kb:       kb,
		pad:      pad,
		touchPad: touchPad,
		pc:       pointer.NewMouseController(tconn),
	}
}

// launchChrome launches the Chrome browser.
func (cl *clamshellActionHandler) launchChrome(ctx context.Context) (time.Time, error) {
	return cuj.LaunchAppFromShelf(ctx, cl.tconn, "Chrome", "Chromium")
}

// newTab creates a new tab of Google Chrome.
// newWindow decide this new tab should open in current Chrome window or open in new Chrome window.
func (cl *clamshellActionHandler) newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
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

// switchWindow switches the Chrome tab from one to another.
func (cl *clamshellActionHandler) switchWindow(ctx context.Context, idxWindow, numWindows int) error {
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

// switchTab switches the Chrome tab from one to another.
func (cl *clamshellActionHandler) switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error {
	return cl.kb.Accel(ctx, "Ctrl+Tab")
}

// pageRefresh refresh a web page (current focus page).
func (cl *clamshellActionHandler) pageRefresh(ctx context.Context) error {
	return cl.kb.Accel(ctx, "refresh")
}

// scrollPage generate the scroll action.
func (cl *clamshellActionHandler) scrollPage() []func(ctx context.Context, conn *chrome.Conn) error {
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
func (cl *clamshellActionHandler) mouseMoveToCenterOfWindow(ctx context.Context, conn *chrome.Conn) error {
	var title string
	if err := conn.Eval(ctx, "document.title", &title); err != nil {
		return errors.Wrap(err, "failed to get current tab's title")
	}

	ui := uiauto.New(cl.tconn)
	window := nodewith.Name(title).Role(role.Window).First()
	if err := ui.WaitForLocation(window)(ctx); err != nil {
		return err
	}

	l, err := ui.Location(ctx, window)
	if err != nil {
		return err
	}

	return cl.pc.Move(ctx, l.CenterPoint(), l.CenterPoint(), 0)
}
