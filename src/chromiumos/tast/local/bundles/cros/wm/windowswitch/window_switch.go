// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package windowswitch contains interface and implementation for switching window.
package windowswitch

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// UIHandler defines the interface for switching window.
type UIHandler interface {
	// SwitchToLRUWindow switches the window to LRU one, LRU as in: Least Recently Used.
	SwitchToLRUWindow(ctx context.Context, windowCnt int) error
	// Close closes the resource of UIHandler.
	Close(ctx context.Context) error
}

type switchDirection int

const (
	// Forward is for keyboardHandler to use Alt+Tab to switch window forward.
	Forward switchDirection = iota
	// Backward is for keyboardHandler to use Alt+Shift+Tab to switch window backward.
	Backward
)

type keyboardHandler struct {
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	dir   switchDirection
}

// NewKeyboardHandler returns a new instance of keyboardHandler.
func NewKeyboardHandler(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, dir switchDirection) *keyboardHandler {
	return &keyboardHandler{
		tconn: tconn,
		kb:    kb,
		dir:   dir,
	}
}

// Close closes the resource of keyboardHandler.
func (k *keyboardHandler) Close(ctx context.Context) error {
	return nil
}

// SwitchToLRUWindow switches the window to LRU one by keyboardHandler.
// There are two directions to switch, forward and backward.
func (k *keyboardHandler) SwitchToLRUWindow(ctx context.Context, windowCnt int) error {
	var (
		cmd         string
		switchTimes int
	)

	switch k.dir {
	case Forward:
		cmd = "Alt"
		switchTimes = windowCnt - 1
	case Backward:
		cmd = "Shift+Alt"
		switchTimes = 1
	}

	ui := uiauto.New(k.tconn)
	// Short pause between key-events, just to make the window cycle view appear longer.
	shortPause := ui.Sleep(200 * time.Millisecond)

	actions := []action.Action{k.kb.AccelPressAction(cmd)}
	for i := 0; i < switchTimes; i++ {
		actions = append(actions,
			shortPause,
			k.kb.AccelPressAction("Tab"),
			shortPause,
			k.kb.AccelReleaseAction("Tab"),
		)
	}
	actions = append(actions, shortPause, k.kb.AccelReleaseAction(cmd))

	if err := uiauto.Combine("switch window by "+cmd+"+Tab", actions...)(ctx); err != nil {
		return err
	}

	return nil
}

type mouseHandler struct {
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	mouse *pointer.MouseContext
}

// NewMouseHandler returns a new instance of mouseHandler.
func NewMouseHandler(tconn *chrome.TestConn, kb *input.KeyboardEventWriter) *mouseHandler {
	return &mouseHandler{
		tconn: tconn,
		kb:    kb,
		mouse: pointer.NewMouse(tconn),
	}
}

// Close closes the resource of mouseHandler.
func (m *mouseHandler) Close(ctx context.Context) error {
	return m.mouse.Close()
}

// SwitchToLRUWindow switches the window to LRU one by mouseHandler.
func (m *mouseHandler) SwitchToLRUWindow(ctx context.Context, windowCnt int) error {
	return switchToLRUWindow(ctx, m.tconn, m.kb, m.mouse, windowCnt)
}

type touchHandler struct {
	tconn *chrome.TestConn
	kb    *input.KeyboardEventWriter
	touch *pointer.TouchContext
}

// NewTouchHandler returns a new instance of touchHandler.
func NewTouchHandler(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) (t *touchHandler, retErr error) {
	touch, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		return nil, err
	}

	return &touchHandler{
		tconn: tconn,
		kb:    kb,
		touch: touch,
	}, nil
}

// Close closes the resource of touchHandler.
func (t *touchHandler) Close(ctx context.Context) error {
	return t.touch.Close()
}

// SwitchToLRUWindow switches the window to LRU one by touchHandler.
func (t *touchHandler) SwitchToLRUWindow(ctx context.Context, windowCnt int) error {
	return switchToLRUWindow(ctx, t.tconn, t.kb, t.touch, windowCnt)
}

// switchToLRUWindow clicks on the rightmost window on the window cycle view.
func switchToLRUWindow(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, pc pointer.Context, windowCnt int) error {
	testing.ContextLog(ctx, "Open the window cycle view")
	if err := uiauto.Combine("open the window cycle view",
		kb.AccelPressAction("Alt+Tab"),
		kb.AccelReleaseAction("Tab"),
	)(ctx); err != nil {
		return err
	}
	defer kb.AccelRelease(ctx, "Alt")

	ui := uiauto.New(tconn)
	altTabWindow := nodewith.Role(role.Window).HasClass("Alt+Tab")
	if err := ui.WaitUntilExists(altTabWindow)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for existence of the window cycle view")
	}

	testing.ContextLog(ctx, "Obtain the rightmost window")
	subWindows := nodewith.Role(role.StaticText).HasClass("Label").Ancestor(altTabWindow)
	nodes, err := ui.NodesInfo(ctx, subWindows)
	if err != nil {
		return errors.Wrap(err, "failed to get the info of window nodes on the window cycle view")
	}
	if len(nodes) != windowCnt {
		return errors.Errorf("unexpected windows count, want: %d, got: %d", windowCnt, len(nodes))
	}

	rightmostLocation := 0
	rightmostName := ""
	for _, node := range nodes {
		if node.Location.Left > rightmostLocation {
			rightmostLocation = node.Location.Left
			rightmostName = node.Name
		}
	}

	testing.ContextLog(ctx, "Select the rightmost window")
	if err := pc.Click(subWindows.Name(rightmostName))(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the rightmost window")
	}

	return nil
}
