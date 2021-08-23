// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wm

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wm/windowswitch"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowSwitch,
		Desc:         "Test window switch functionality through window cycle view by key-command, mouse click and touch tap",
		Contacts:     []string{"sun.tsai@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

type switchType int

const (
	switchByKeyboard switchType = iota
	switchByKeyboardBackward
	switchByMouse
	switchByTouch
)

// WindowSwitch tests window switch functionality through window cycle view by key-command, mouse click and touch tap.
func WindowSwitch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard input: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Expecting three windows,
	// one tab for first window, two tabs for second window, three tabs for third window.
	tabs := [][]*tab{
		{
			&tab{url: "https://www.google.com"},
		}, {
			&tab{url: "https://www.youtube.com"},
			&tab{url: "https://www.facebook.com"},
		}, {
			&tab{url: "https://twitter.com"},
			&tab{url: "https://www.wikipedia.org"},
			&tab{url: "https://www.netflix.com"},
		},
	}

	ui := uiauto.New(tconn)
	maximizeBtnFinder := nodewith.Name("Maximize").Role(role.Button).HasClass("FrameCaptionButton")

	// Keep the order of all windows to verify the window is
	// swtiched to expected window after switch window action.
	var orderedWindows []int

	// Open tabs.
	for windowIdx, window := range tabs {
		for tabIdx, tab := range window {
			if err := tab.open(ctx, cr, kb, tabIdx == 0); err != nil {
				s.Fatal("Failed to open tab: ", err)
			}
			defer tab.close(cleanupCtx)

			testing.ContextLogf(ctx, "Open new tab [%s] at window [%d]", tab.url, windowIdx)
		}

		// Maximize the Chrome window to make sure the correct window is focused when verifying the tab is rendered.
		if err := ui.IfSuccessThen(
			ui.WithTimeout(2*time.Second).WaitUntilExists(maximizeBtnFinder),
			ui.LeftClick(maximizeBtnFinder),
		)(ctx); err != nil {
			s.Fatal("Failed to maximize Chrome window: ", err)
		}

		// Uses the index as ordered id.
		// The newly opened window will be at the front.
		orderedWindows = append([]int{windowIdx}, orderedWindows...)
	}

	defer func(ctx context.Context) {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
	}(cleanupCtx)

	if err := switchAndVerify(ctx, tconn, kb, tabs, orderedWindows, switchByKeyboard); err != nil {
		s.Fatal("Failed to switch forward and verify by keyboard: ", err)
	}
	if err := switchAndVerify(ctx, tconn, kb, tabs, orderedWindows, switchByKeyboardBackward); err != nil {
		s.Fatal("Failed to switch backward and verify by keyboard: ", err)
	}
	if err := switchAndVerify(ctx, tconn, kb, tabs, orderedWindows, switchByMouse); err != nil {
		s.Fatal("Failed to switch and verify by mouse: ", err)
	}
	if err := switchAndVerify(ctx, tconn, kb, tabs, orderedWindows, switchByTouch); err != nil {
		s.Fatal("Failed to switch and verify by touch: ", err)
	}
}

type tab struct {
	url  string
	conn *chrome.Conn
}

func (t *tab) open(ctx context.Context, cr *chrome.Chrome, kb *input.KeyboardEventWriter, isNewWindow bool) (err error) {
	if isNewWindow {
		t.conn, err = cr.NewConn(ctx, t.url, cdputil.WithNewWindow())
		return err
	}

	if err = kb.Accel(ctx, "Ctrl+T"); err != nil {
		return errors.Wrap(err, "failed to use Ctrl+T to open a new tab")
	}

	if t.conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
		return errors.Wrap(err, "failed to find new tab")
	}
	if err = t.conn.Navigate(ctx, t.url); err != nil {
		// New tab has been opened, need to close it if navigation failed.
		t.close(ctx)

		return errors.Wrapf(err, "failed to navigate to %s", t.url)
	}

	return nil
}

func (t *tab) close(ctx context.Context) {
	t.conn.CloseTarget(ctx)
	t.conn.Close()
}

func switchAndVerify(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, tabs [][]*tab, orderedWindows []int, st switchType) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	var handler windowswitch.UIHandler
	var err error

	switch st {
	case switchByKeyboard:
		handler = windowswitch.NewKeyboardHandler(tconn, kb, windowswitch.Forward)
	case switchByKeyboardBackward:
		handler = windowswitch.NewKeyboardHandler(tconn, kb, windowswitch.Backward)
	case switchByMouse:
		handler = windowswitch.NewMouseHandler(tconn, kb)
	case switchByTouch:
		if handler, err = windowswitch.NewTouchHandler(ctx, tconn, kb); err != nil {
			return errors.Wrap(err, "failed to create touch handler")
		}
	}
	defer handler.Close(cleanupCtx)

	// Switch over through all windows.
	for range tabs {
		if err := handler.SwitchToLRUWindow(ctx, len(orderedWindows)); err != nil {
			return errors.Wrap(err, "failed to switch window")
		}
		testing.ContextLog(ctx, "Window switched")

		if err := updateWindowOrderAndVerify(ctx, &orderedWindows, tabs); err != nil {
			return errors.Wrap(err, "failed to verify if the window is switched")
		}
	}

	return nil
}

func updateWindowOrderAndVerify(ctx context.Context, orderedWindows *[]int, allTabs [][]*tab) error {
	if orderedWindows == nil || len(*orderedWindows) < 1 {
		return errors.New("invalid ordered windows")
	}
	lastWindow := len(*orderedWindows) - 1
	*orderedWindows = append([]int{(*orderedWindows)[lastWindow]}, (*orderedWindows)[:lastWindow]...)
	testing.ContextLog(ctx, "Window order updated, expected order: ", orderedWindows)

	focusedWindow := allTabs[(*orderedWindows)[0]] // Current focused window will always the first one.
	lastTab := len(focusedWindow) - 1
	return webutil.WaitForRender(ctx, focusedWindow[lastTab].conn, 10*time.Second)
}
