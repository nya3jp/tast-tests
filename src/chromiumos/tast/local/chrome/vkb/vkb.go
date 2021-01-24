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
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/testing"
)

// ShowVirtualKeyboard forces the virtual keyboard show up via Chrome API.
// It is not recommended to use on testing a real user input through the virtual keyboard.
// Virtual keyboard should be normally triggered by focusing an input field.
// Usage: It can be used to test Layout and UI interaction in a quick way.
// For example, testing switch layout.
func ShowVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Eval(ctx, `tast.promisify(chrome.inputMethodPrivate.showInputView)()`, nil)
}

// HideVirtualKeyboard forces the virtual keyboard to be hidden via Chrome API.
// It is not recommended to use on testing a real user input through the virtual keyboard.
// Virtual keyboard should be normally triggered by defocusing an input field.
// Usage: It can be used in test cleanup.
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

// IsShown checks if the virtual keyboard is currently shown. It checks whether
// there is a visible DOM element with an accessibility role of "keyboard".
func IsShown(ctx context.Context, tconn *chrome.TestConn) (shown bool, err error) {
	params := ui.FindParams{
		Role: ui.RoleTypeRootWebArea,
		Name: "Chrome OS Virtual Keyboard",
	}
	return ui.Exists(ctx, tconn, params)
}

// WaitLocationStable waits for the virtual keyboard to appear and have a stable location.
func WaitLocationStable(ctx context.Context, tconn *chrome.TestConn) error {
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

// TapKey simulates a mouse click event on the middle of the specified key via touch event. The key can
// be any letter of the alphabet, "space" or "backspace".
func TapKey(ctx context.Context, tconn *chrome.TestConn, keyName string) error {
	key, err := FindKeyNode(ctx, tconn, keyName)
	if err != nil {
		return errors.Wrapf(err, "failed to find key: %s", keyName)
	}
	defer key.Release(ctx)

	if err := mouse.Move(ctx, tconn, key.Location.CenterPoint(), 10*time.Millisecond); err != nil {
		return errors.Wrapf(err, "failed to move mouse to key %s", keyName)
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		return errors.Wrapf(err, "failed to press key %s: ", keyName)
	}
	testing.Sleep(ctx, 50*time.Millisecond)
	return mouse.Release(ctx, tconn, mouse.LeftButton)
}

// FindKeyNode returns the ui node of the specified key.
func FindKeyNode(ctx context.Context, tconn *chrome.TestConn, keyName string) (*ui.Node, error) {
	keyParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: keyName,
	}

	return DescendantNode(ctx, tconn, keyParams)
}

// DescendantNode returns the first descendant node in virtual keyboard matches given FindParams.
func DescendantNode(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) (*ui.Node, error) {
	vk, err := VirtualKeyboard(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find virtual keyboard automation node")
	}
	defer vk.Release(ctx)

	node, err := vk.DescendantWithTimeout(ctx, params, 10*time.Second)
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

// SetFloatingMode changes the virtual keyboard to floating/dock layout via private javascript function.
func SetFloatingMode(ctx context.Context, cr *chrome.Chrome, enable bool) error {
	bconn, err := BackgroundConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to create IME background connection")
	}
	defer bconn.Close()

	if err := bconn.WaitForExpr(ctx, fmt.Sprintf("background.inputviewLoader.module$exports$google3$i18n$input$javascript$chos$loader_Loader_prototype$controller.maybeSetFloatingModeEnabled(%t)", enable)); err != nil {
		if enable {
			return errors.Wrap(err, "failed to wait for virtual keyboard to be floating mode")
		}
		return errors.Wrap(err, "failed to wait for virtual keyboard to be dock mode")
	}
	return nil
}

// TapKeys simulates tap events in the middle of the specified sequence of keys via touch event.
// Each key can be any letter of the alphabet, "space" or "backspace".
func TapKeys(ctx context.Context, tconn *chrome.TestConn, keys []string) error {
	for _, key := range keys {
		if err := TapKey(ctx, tconn, key); err != nil {
			return err
		}
		testing.Sleep(ctx, 100*time.Millisecond)
	}
	return nil
}

// WaitForKeysExist waits for a list of keys to appear on virtual keyboard.
// Note: Should not use FindKeyNode in a loop to implement this function, because it waits for each key within a timeout.
func WaitForKeysExist(ctx context.Context, tconn *chrome.TestConn, keys []string) error {
	vk, err := VirtualKeyboard(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find virtual keyboard automation node")
	}
	defer vk.Release(ctx)

	return testing.Poll(ctx, func(ctx context.Context) error {
		var notFoundKeys []string
		for _, key := range keys {
			keyParams := ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: key,
			}

			keyExists, err := vk.DescendantExists(ctx, keyParams)
			if err != nil {
				return errors.Wrapf(err, "failed to find key node %s", key)
			}

			if !keyExists {
				notFoundKeys = append(notFoundKeys, key)
			}
		}
		if len(notFoundKeys) > 0 {
			return errors.Errorf("these keys are not found: %v", notFoundKeys)
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second})
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
	// TODO(b/157686038) A better solution to identify decoder status.
	// Decoder works async in returning status to frontend IME and self loading.
	// Using sleep temporarily before a reliable evaluation api provided in cl/339837443.
	testing.Sleep(ctx, 10*time.Second)
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
	return WaitLocationStable(ctx, tconn)
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
	if err := WaitLocationStable(ctx, tconn); err != nil {
		return err
	}

	return WaitForDecoderEnabled(ctx, cr, true)
}

// SwitchToVoiceInput changes virtual keyboard to voice input layout.
func SwitchToVoiceInput(ctx context.Context, tconn *chrome.TestConn) error {
	if err := TapKey(ctx, tconn, "Voice"); err != nil {
		return errors.Wrap(err, "failed to tap voice input button")
	}

	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "Got it",
		ClassName: "voice-got-it",
	}
	opts := testing.PollOptions{Timeout: 3 * time.Second, Interval: 500 * time.Millisecond}
	return ui.StableFindAndClick(ctx, tconn, params, &opts)
}

// TapKeyboardInput changes virtual keyboard to keyboard input layout.
func TapKeyboardInput(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "Back",
		ClassName: "sk icon-key",
	}
	opts := testing.PollOptions{Timeout: 2 * time.Second}
	return ui.StableFindAndClick(ctx, tconn, params, &opts)
}

// TapAccessPoints changes the suggestion bar to input icons.
func TapAccessPoints(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "Show access points",
		ClassName: "sk icon-key",
	}
	opts := testing.PollOptions{Timeout: 2 * time.Second}
	return ui.StableFindAndClick(ctx, tconn, params, &opts)
}

// TapHandwritingInputAndWaitForEngine changes virtual keyboard to handwriting input layout and waits for the handwriting
// engine to become ready.
func TapHandwritingInputAndWaitForEngine(ctx context.Context, tconn *chrome.TestConn) error {
	// TODO(crbug/1165424): Check if handwriting input engine is ready.
	// Wait for the handwriting input to become ready to take in the handwriting.
	// If a stroke is completed before the handwriting input is ready, the stroke will not be recognized.
	defer testing.Sleep(ctx, 1*time.Second)

	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "switch to handwriting, not compatible with ChromeVox",
		ClassName: "sk icon-key",
	}
	opts := testing.PollOptions{Timeout: 2 * time.Second}
	return ui.StableFindAndClick(ctx, tconn, params, &opts)
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
