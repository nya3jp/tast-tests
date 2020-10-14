// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkb contains shared code to interact with the virtual keyboard.
package vkb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// ImePrefix is the prefix of IME chrome extension
const ImePrefix = "_comp_ime_jkghodnilhceideoidjikpgommlajknk"

// ShowVirtualKeyboard forces the virtual keyboard to open.
func ShowVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Eval(ctx, `tast.promisify(chrome.inputMethodPrivate.showInputView)()`, nil)
}

// HideVirtualKeyboard forces the virtual keyboard to be hidden.
func HideVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Eval(ctx, `tast.promisify(chrome.inputMethodPrivate.hideInputView)()`, nil); err != nil {
		return errors.Wrap(err, "failed to call hide inputview api")
	}
	return WaitUntilHidden(ctx, tconn)
}

// VirtualKeyboard returns a reference to chrome.automation API AutomationNode of virtual keyboard.
func VirtualKeyboard(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		Role: ui.RoleTypeRootWebArea,
		Name: "Chrome OS Virtual Keyboard",
	}
	return ui.FindWithTimeout(ctx, tconn, params, 30*time.Second)
}

// SetCurrentInputMethod sets the current input method used by the virtual
// keyboard.
func SetCurrentInputMethod(ctx context.Context, tconn *chrome.TestConn, inputMethod string) error {
	if err := tconn.Call(ctx, nil, `async (ime) => {
		await tast.promisify(chrome.autotestPrivate.setWhitelistedPref)(
		    'settings.language.preload_engines', ime);
		await tast.promisify(chrome.inputMethodPrivate.setCurrentInputMethod)(ime);
	}`, ImePrefix+inputMethod); err != nil {
		return errors.Wrapf(err, "failed to set current input method: %q", inputMethod)
	}

	// Change language via tconn requiring a few seconds to install.
	// TODO(b/157686038): Use API to identify completion of changing language
	testing.Sleep(ctx, 3*time.Second)
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}
	return nil
}

// GetCurrentInputMethod returns the current input method id used by the virtual
// keyboard.
func GetCurrentInputMethod(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var id string
	if err := tconn.Call(ctx, &id, `tast.promisify(chrome.inputMethodPrivate.getCurrentInputMethod)`); err != nil {
		return "", errors.Wrap(err, "failed to get current input method")
	}

	return strings.TrimPrefix(id, ImePrefix), nil
}

// IsShown checks if the virtual keyboard is currently shown. It checks whether
// there is a visible DOM element with an accessibility role of "keyboard".
func IsShown(ctx context.Context, tconn *chrome.TestConn) (shown bool, err error) {
	params := ui.FindParams{
		Role: ui.RoleTypeRootWebArea,
		Name: "Chrome OS Virtual Keyboard",
	}
	return ui.Exists(ctx, tconn, params)
}

// WaitUntilShown waits for the virtual keyboard to appear. It waits keyboard appears in A11y tree and locationed.
func WaitUntilShown(ctx context.Context, tconn *chrome.TestConn) error {
	keyboard, err := VirtualKeyboard(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "fail to wait for virtual keyboard shown")
	}
	defer keyboard.Release(ctx)
	return keyboard.WaitLocationStable(ctx, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 20 * time.Second})
}

// WaitUntilHidden waits for the virtual keyboard to hide. It waits until the node is gone from a11y tree.
func WaitUntilHidden(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if shown, err := IsShown(ctx, tconn); err != nil {
			return testing.PollBreak(err)
		} else if shown {
			return errors.New("waiting for virtual keyboard to be hidden")
		}
		return nil
	}, &testing.PollOptions{Interval: 2 * time.Second, Timeout: 20 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for virtual keyboard to be hidden")
	}
	return nil
}

// WaitUntilButtonsRender waits for the virtual keyboard to render some buttons.
// Deprecated. This function does not work for non-EN input view or non-letter layouts.
// It is not actually required because TapKey will find the key node first.
func WaitUntilButtonsRender(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		keyboard, err := ui.Find(ctx, tconn, ui.FindParams{Role: ui.RoleTypeKeyboard})
		if err != nil {
			return errors.Wrap(err, "virtual keyboard does not exist yet")
		}
		defer keyboard.Release(ctx)
		keys, err := keyboard.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeButton})
		if err != nil {
			return errors.Wrap(err, "keyboard buttons don't exist yet")
		}
		defer keys.Release(ctx)
		// English keyboard should have at least 26 keys.
		if len(keys) <= 26 {
			return errors.New("not all buttons have rendered yet")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "failed to wait for virtual keyboad buttons to render")
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for animation finished")
	}
	return nil
}

// UIConn returns a connection to the virtual keyboard HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The connection is lazily created, and this function will block until the
// extension is loaded or ctx's deadline is reached. The caller should close
// the returned connection.
func UIConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extURLPrefix := "chrome-extension://jkghodnilhceideoidjikpgommlajknk/inputview.html"
	f := func(t *target.Info) bool { return strings.HasPrefix(t.URL, extURLPrefix) }
	return c.NewConnForTarget(ctx, f)
}

// BackgroundConn returns a connection to the virtual keyboard background page,
// where JavaScript can be executed to simulate interactions with IME.
func BackgroundConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extURL := "chrome-extension://jkghodnilhceideoidjikpgommlajknk/background.html"

	// Background target from login persists for a few seconds, causing 2 background targets.
	// Polling until connected to the unique target.
	var bconn *chrome.Conn
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		bconn, err = c.NewConnForTarget(ctx, chrome.MatchTargetURL(extURL))
		return err
	}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 3 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for unique virtual keyboard background target")
	}

	return bconn, nil
}

// TODO(b/159657128): Investigate why TapKey does not work very well consistently.

// TapKey simulates a tap event on the middle of the specified key via touch event. The key can
// be any letter of the alphabet, "space" or "backspace".
func TapKey(ctx context.Context, tconn *chrome.TestConn, keyName string) error {
	key, err := FindKeyNode(ctx, tconn, keyName)
	if err != nil {
		return errors.Wrapf(err, "failed to find key: %s", keyName)
	}

	if err := key.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click key %s", keyName)
	}
	return nil
}

// FindKeyNode returns the ui node of the specified key.
func FindKeyNode(ctx context.Context, tconn *chrome.TestConn, keyName string) (*ui.Node, error) {
	keyParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: keyName,
	}

	return DescendantNode(ctx, tconn, keyParams)
}

// DescendantNode returns the first descendant node matches given FindParams.
func DescendantNode(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (*ui.Node, error) {
	vk, err := VirtualKeyboard(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find virtual keyboad automation node")
	}
	defer vk.Release(ctx)

	node, err := vk.DescendantWithTimeout(ctx, params, 5*time.Second)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find descendant node with %v", params)
	}
	return node, nil
}

// TapKeyJS simulates a tap event on the middle of the specified key via javascript. The key can
// be any letter of the alphabet, "space" or "backspace".
func TapKeyJS(ctx context.Context, kconn *chrome.Conn, key string) error {
	return kconn.Call(ctx, nil, `(key) => {
		// Multiple keys can have the same aria label but only one is visible.
		const keys = document.querySelectorAll('[aria-label=' + key + ']')
		if (!keys) {
			throw new Error('Key ' + key + ' not found. No element with aria-label ' + key +'.');
		}
		for (const key of keys) {
			const rect = key.getBoundingClientRect();
			if (rect.width <= 0 || rect.height <= 0) {
				continue;
			}
			const e = new Event('pointerdown');
			e.clientX = rect.x + rect.width / 2;
			e.clientY = rect.y + rect.height / 2;
			key.dispatchEvent(e);
			key.dispatchEvent(new Event('pointerup'));
			return;
		}
		throw new Error('Key ' + key + ' not clickable. Found elements with aria-label ' + key + ', but they were not visible.');
	}`, key)
}

// SetFloatingMode changes virtual keyboard to floating/dock layout.
func SetFloatingMode(ctx context.Context, cr *chrome.Chrome, enable bool) error {
	bconn, err := BackgroundConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to create IME background connection")
	}
	defer bconn.Close()

	if err := bconn.WaitForExpr(ctx, fmt.Sprintf("background.inputviewLoader_.controller_.maybeSetFloatingModeEnabled(%t)", enable)); err != nil {
		if enable {
			return errors.Wrap(err, "failed to wait for virtual keyboard to be floating mode")
		}
		return errors.Wrap(err, "failed to wait for virtual keyboard to be dock mode")
	}
	return nil
}

// TapKeys simulates tap events on the middle of the specified sequence of keys via touch event.
// Each keys can be any letter of the alphabet, "space" or "backspace".
func TapKeys(ctx context.Context, tconn *chrome.TestConn, keys []string) error {
	for _, key := range keys {
		if err := TapKey(ctx, tconn, key); err != nil {
			return err
		}
		testing.Sleep(ctx, 100*time.Millisecond)
	}
	return nil
}

// TapKeysJS simulates tap events on the middle of the specified sequence of keys via javascript.
// Each keys can be any letter of the alphabet, "space" or "backspace".
func TapKeysJS(ctx context.Context, kconn *chrome.Conn, keys []string) error {
	for _, key := range keys {
		if err := TapKeyJS(ctx, kconn, key); err != nil {
			return err
		}
		testing.Sleep(ctx, 100*time.Millisecond)
	}
	return nil
}

// GetSuggestions returns suggestions that are currently displayed by the
// virtual keyboard.
func GetSuggestions(ctx context.Context, kconn *chrome.Conn) ([]string, error) {
	var suggestions []string
	err := kconn.Eval(ctx, `
	(() => {
		const elems = document.querySelectorAll('.candidate-span');
		return Array.prototype.map.call(elems, x => x.textContent);
	})()
`, &suggestions)
	return suggestions, err
}

// WaitForDecoderEnabled waits for decoder to be enabled or disabled.
func WaitForDecoderEnabled(ctx context.Context, cr *chrome.Chrome, enabled bool) error {
	bconn, err := BackgroundConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to create IME background connection")
	}
	defer bconn.Close()

	// TODO(b/157686038) A better solution to identify decoder status.
	// Decoder works async in returning status to frontend IME and self loading.
	// So sleep is still required to wait for decoder warming up.
	if err := bconn.WaitForExpr(ctx, fmt.Sprintf("background.inputviewLoader_.controller_.currentInputBundle_.ime_.shouldUseDecoder()===%t", enabled)); err != nil {
		return errors.Wrapf(err, "failed wait for decoder enabled to be %t: %v", enabled, err)
	}

	if enabled {
		return testing.Sleep(ctx, 3*time.Second)
	}
	return nil
}

// ClickUntilVKShown repeatedly left clicks the node until the condition returns true with no error.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// The interval between clicks and the timeout can be specified using testing.PollOptions.
func ClickUntilVKShown(ctx context.Context, tconn *chrome.TestConn, node *ui.Node) error {
	condition := func(ctx context.Context) (bool, error) {
		return IsShown(ctx, tconn)
	}
	opts := testing.PollOptions{Timeout: 30 * time.Second, Interval: 3 * time.Second}
	if err := node.LeftClickUntil(ctx, condition, &opts); err != nil {
		return errors.Wrapf(err, "failed to click %v until vk shown", node)
	}
	return nil
}

// FindAndClickUntilVKShown is similar to ClickUntilVKShown.
// It finds element first and then performs ClickUntilVKShown.
func FindAndClickUntilVKShown(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, 20*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find node with params %v", params)
	}
	defer node.Release(ctx)
	return ClickUntilVKShown(ctx, tconn, node)
}

// WaitForVKReady waits for virtual keyboard shown, completely positioned and decoder ready for use.
// Similar to document.readyState === 'complete' in DOM, Virtual keyboard's readiness needs to be ensured before using it.
func WaitForVKReady(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	if err := WaitUntilShown(ctx, tconn); err != nil {
		return err
	}

	return WaitForDecoderEnabled(ctx, cr, true)
}

// EnableA11yVirtualKeyboard enables or disables accessibility mode of the
// virtual keyboard. When disabled, the tablet non-a11y virtual keyboard will
// be used when activated.
func EnableA11yVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.a11y.virtual_keyboard", enabled)
}

// SelectFromSuggestion waits for suggestion candidate to appear and clicks it to select.
func SelectFromSuggestion(ctx context.Context, tconn *chrome.TestConn, candidateText string) error {
	candidateFindParams := ui.FindParams{
		Role:      ui.RoleTypeButton,
		ClassName: "sk",
		Name:      candidateText,
	}

	opts := testing.PollOptions{Timeout: 3 * time.Second, Interval: 500 * time.Millisecond}
	return ui.StableFindAndClick(ctx, tconn, candidateFindParams, &opts)
}
