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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// ShowVirtualKeyboard forces the virtual keyboard show up via Chrome API.
// It is not recommended to use on testing a real user input through the virtual keyboard.
// Virtual keyboard should be normally triggered by focusing an input field.
// Usage: It can be used to test Layout and UI interaction in a quick way.
// For example, testing switch layout.
func ShowVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn) error {
	if err := tconn.Eval(ctx, `tast.promisify(chrome.inputMethodPrivate.showInputView)()`, nil); err != nil {
		return errors.Wrap(err, "failed to show the virtual keyboard")
	}
	return nil
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

func vkRootFinder() *nodewith.Finder {
	return nodewith.Role(role.RootWebArea).Name("Chrome OS Virtual Keyboard")
}

// VirtualKeyboard returns a reference to chrome.automation API AutomationNode of virtual keyboard.
func VirtualKeyboard(ctx context.Context, tconn *chrome.TestConn) (*uiauto.NodeInfo, error) {
	return uiauto.New(tconn).WithTimeout(30*time.Second).Info(ctx, vkRootFinder())
}

// IsShown checks if the virtual keyboard is currently shown. It checks whether
// there is a visible DOM element with an accessibility role of "keyboard".
func IsShown(ctx context.Context, tconn *chrome.TestConn) bool {
	return uiauto.New(tconn).Exists(vkRootFinder())(ctx) == nil
}

// WaitLocationStable waits for the virtual keyboard to appear and have a stable location.
func WaitLocationStable(ctx context.Context, tconn *chrome.TestConn) error {
	_, err := VirtualKeyboard(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "fail to wait for virtual keyboard shown")
	}
	_, err = uiauto.New(tconn).WithTimeout(20*time.Second).WithInterval(1*time.Second).Location(ctx, vkRootFinder())
	return err
}

// WaitUntilHidden waits for the virtual keyboard to hide. It waits until the node is gone from a11y tree.
func WaitUntilHidden(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if IsShown(ctx, tconn) {
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
		keys, err := uiauto.New(tconn).NodesInfo(ctx, nodewith.Role(role.Button).Ancestor(nodewith.Role(role.Keyboard)))
		if err != nil {
			return err
		}
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

	// Note: Must use mouse Move + Press + Sleep + Release here instead of Click.
	// Mouse click is simulated by calling Chrome private api `chrome.autotestPrivate.mouseClick`.
	// It works for most cases except virtual keyboard.
	// In vkb extension, it listens to keyPress to send vk layout event to decoder
	// before sending the actual key tap event.
	// Mouse click is too quick and causes a racing issue that decoder receives tap key without layout info.
	if err := mouse.Move(ctx, tconn, key.Location.CenterPoint(), 10*time.Millisecond); err != nil {
		return errors.Wrapf(err, "failed to move mouse to key %s", keyName)
	}
	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		return errors.Wrapf(err, "failed to press key %s: ", keyName)
	}
	testing.Sleep(ctx, 50*time.Millisecond)
	return mouse.Release(ctx, tconn, mouse.LeftButton)
}

func keyFinder(key string) *nodewith.Finder {
	return nodewith.Role(role.Button).Name(key).Ancestor(vkRootFinder())
}

// FindKeyNode returns the ui node of the specified key.
func FindKeyNode(ctx context.Context, tconn *chrome.TestConn, keyName string) (*uiauto.NodeInfo, error) {
	return DescendantNode(ctx, tconn, keyFinder(keyName))
}

// DescendantNode returns the first descendant node in virtual keyboard matches given FindParams.
func DescendantNode(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder) (*uiauto.NodeInfo, error) {
	node, err := uiauto.New(tconn).WithTimeout(10*time.Second).Info(ctx, finder.Ancestor(vkRootFinder()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find descendant node under the virtual keyboard")
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
	ui := uiauto.New(tconn).WithInterval(1 * time.Second).WithTimeout(10 * time.Second)
	if err := ui.Poll(func(ctx context.Context) error {
		var notFoundKeys []string
		for _, key := range keys {
			if ui.Exists(keyFinder(key))(ctx) != nil {
				notFoundKeys = append(notFoundKeys, key)
			}
		}
		if len(notFoundKeys) > 0 {
			return errors.Errorf("these keys are not found: %v", notFoundKeys)
		}
		return nil
	})(ctx); err != nil {
		return errors.Wrapf(err, "while waiting for these keys to exist: %v", keys)
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
	// TODO(b/157686038) A better solution to identify decoder status.
	// Decoder works async in returning status to frontend IME and self loading.
	// Using sleep temporarily before a reliable evaluation api provided in cl/339837443.
	testing.Sleep(ctx, 10*time.Second)
	return nil
}

// ClickUntilVKShown repeatedly left clicks the node until the condition returns true with no error.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// The interval between clicks and the timeout can be specified using testing.PollOptions.
func ClickUntilVKShown(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder) error {
	ui := uiauto.New(tconn)
	return uiauto.Combine("click until vk shown",
		ui.WithTimeout(20*time.Second).WaitUntilExists(finder),
		ui.WithTimeout(30*time.Second).WithInterval(3*time.Second).LeftClickUntil(finder, func(ctx context.Context) error {
			if IsShown(ctx, tconn) {
				return nil
			}
			return errors.New("node not shown missing")
		}),
		WaitLocationStableAction(tconn),
	)(ctx)
}

// WaitForVKReady waits for virtual keyboard shown, completely positioned and decoder ready for use.
// Similar to document.readyState === 'complete' in DOM, Virtual keyboard's readiness needs to be ensured before using it.
func WaitForVKReady(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	return uiauto.Combine("wait for virtual keyboard to be ready",
		WaitLocationStableAction(tconn),
		WaitForDecoderEnabledAction(cr, true))(ctx)
}

// SwitchToVoiceInput changes virtual keyboard to voice input layout.
func SwitchToVoiceInput(ctx context.Context, tconn *chrome.TestConn) error {
	if err := TapKey(ctx, tconn, "Voice"); err != nil {
		return errors.Wrap(err, "failed to tap voice input button")
	}

	finder := nodewith.Role(role.Button).Name("Got it").ClassName("voice-got-it")
	return uiauto.New(tconn).WithTimeout(3 * time.Second).WithInterval(500 * time.Millisecond).LeftClick(finder)(ctx)
}

// TapKeyboardInput changes virtual keyboard to keyboard input layout.
func TapKeyboardInput(ctx context.Context, tconn *chrome.TestConn) error {
	finder := nodewith.Role(role.Button).Name("Back").ClassName("sk icon-key")
	return uiauto.New(tconn).WithTimeout(2 * time.Second).LeftClick(finder)(ctx)
}

// TapAccessPoints changes the suggestion bar to input icons.
func TapAccessPoints(ctx context.Context, tconn *chrome.TestConn) error {
	finder := nodewith.Role(role.Button).Name("Show access points").ClassName("sk icon-key")
	return uiauto.New(tconn).WithTimeout(2 * time.Second).LeftClick(finder)(ctx)
}

// TapHandwritingInputAndWaitForEngine changes virtual keyboard to handwriting input layout and waits for the handwriting
// engine to become ready.
func TapHandwritingInputAndWaitForEngine(ctx context.Context, tconn *chrome.TestConn) error {
	// TODO(crbug/1165424): Check if handwriting input engine is ready.
	// Wait for the handwriting input to become ready to take in the handwriting.
	// If a stroke is completed before the handwriting input is ready, the stroke will not be recognized.
	defer testing.Sleep(ctx, 1*time.Second)

	finder := nodewith.Role(role.Button).Name("switch to handwriting, not compatible with ChromeVox").ClassName("sk icon-key")
	return uiauto.New(tconn).WithTimeout(2 * time.Second).LeftClick(finder)(ctx)
}

// EnableA11yVirtualKeyboard enables or disables accessibility mode of the
// virtual keyboard. When disabled, the tablet non-a11y virtual keyboard will
// be used when activated.
func EnableA11yVirtualKeyboard(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.a11y.virtual_keyboard", enabled)
}

// SelectFromSuggestion waits for suggestion candidate to appear and clicks it to select.
func SelectFromSuggestion(ctx context.Context, tconn *chrome.TestConn, candidateText string) error {
	finder := nodewith.Role(role.Button).Name(candidateText).ClassName("sk")
	return uiauto.New(tconn).WithTimeout(3 * time.Second).WithInterval(500 * time.Millisecond).LeftClick(finder)(ctx)
}

// ShowVirtualKeyboardAction returns a uiauto.Action which calls ShowVirtualKeyboard.
func ShowVirtualKeyboardAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.ShowVirtualKeyboardAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return ShowVirtualKeyboard(ctx, tconn) })
}

// HideVirtualKeyboardAction returns a uiauto.Action which calls HideVirtualKeyboard.
func HideVirtualKeyboardAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.HideVirtualKeyboardAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return HideVirtualKeyboard(ctx, tconn) })
}

// WaitLocationStableAction returns a uiauto.Action which calls WaitLocationStable.
func WaitLocationStableAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.WaitLocationStableAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return WaitLocationStable(ctx, tconn) })
}

// WaitUntilHiddenAction returns a uiauto.Action which calls WaitUntilHidden.
func WaitUntilHiddenAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.WaitUntilHiddenAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return WaitUntilHidden(ctx, tconn) })
}

// WaitUntilButtonsRenderAction returns a uiauto.Action which calls WaitUntilButtonsRender.
func WaitUntilButtonsRenderAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.WaitUntilButtonsRenderAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return WaitUntilButtonsRender(ctx, tconn) })
}

// TapKeyAction returns a uiauto.Action which calls TapKey.
func TapKeyAction(tconn *chrome.TestConn, keyName string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.TapKeyAction(tconn *chrome.TestConn, keyName string) with keyName=%v)", keyName),
		func(ctx context.Context) error { return TapKey(ctx, tconn, keyName) })
}

// TapKeyJSAction returns a uiauto.Action which calls TapKeyJS.
func TapKeyJSAction(kconn *chrome.Conn, key string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.TapKeyJSAction(kconn *chrome.Conn, key string) with key=%v)", key),
		func(ctx context.Context) error { return TapKeyJS(ctx, kconn, key) })
}

// SetFloatingModeAction returns a uiauto.Action which calls SetFloatingMode.
func SetFloatingModeAction(cr *chrome.Chrome, enable bool) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.SetFloatingModeAction(cr *chrome.Chrome, enable bool) with enable=%v)", enable),
		func(ctx context.Context) error { return SetFloatingMode(ctx, cr, enable) })
}

// TapKeysAction returns a uiauto.Action which calls TapKeys.
func TapKeysAction(tconn *chrome.TestConn, keys []string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.TapKeysAction(tconn *chrome.TestConn, keys []string) with keys=%v)", keys),
		func(ctx context.Context) error { return TapKeys(ctx, tconn, keys) })
}

// WaitForKeysExistAction returns a uiauto.Action which calls WaitForKeysExist.
func WaitForKeysExistAction(tconn *chrome.TestConn, keys []string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.WaitForKeysExistAction(tconn *chrome.TestConn, keys []string) with keys=%v)", keys),
		func(ctx context.Context) error { return WaitForKeysExist(ctx, tconn, keys) })
}

// TapKeysJSAction returns a uiauto.Action which calls TapKeysJS.
func TapKeysJSAction(kconn *chrome.Conn, keys []string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.TapKeysJSAction(kconn *chrome.Conn, keys []string) with keys=%v)", keys),
		func(ctx context.Context) error { return TapKeysJS(ctx, kconn, keys) })
}

// WaitForDecoderEnabledAction returns a uiauto.Action which calls WaitForDecoderEnabled.
func WaitForDecoderEnabledAction(cr *chrome.Chrome, enabled bool) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.WaitForDecoderEnabledAction(cr *chrome.Chrome, enabled bool) with enabled=%v)", enabled),
		func(ctx context.Context) error { return WaitForDecoderEnabled(ctx, cr, enabled) })
}

// ClickUntilVKShownAction returns a uiauto.Action which calls ClickUntilVKShown.
func ClickUntilVKShownAction(tconn *chrome.TestConn, finder *nodewith.Finder) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.ClickUntilVKShownAction(tconn *chrome.TestConn, finder *nodewith.Finder) with finder=%+v)", finder),
		func(ctx context.Context) error { return ClickUntilVKShown(ctx, tconn, finder) })
}

// WaitForVKReadyAction returns a uiauto.Action which calls WaitForVKReady.
func WaitForVKReadyAction(tconn *chrome.TestConn, cr *chrome.Chrome) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.WaitForVKReadyAction(tconn *chrome.TestConn, cr *chrome.Chrome) with )",
		func(ctx context.Context) error { return WaitForVKReady(ctx, tconn, cr) })
}

// SwitchToVoiceInputAction returns a uiauto.Action which calls SwitchToVoiceInput.
func SwitchToVoiceInputAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.SwitchToVoiceInputAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return SwitchToVoiceInput(ctx, tconn) })
}

// TapKeyboardInputAction returns a uiauto.Action which calls TapKeyboardInput.
func TapKeyboardInputAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.TapKeyboardInputAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return TapKeyboardInput(ctx, tconn) })
}

// TapAccessPointsAction returns a uiauto.Action which calls TapAccessPoints.
func TapAccessPointsAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.TapAccessPointsAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return TapAccessPoints(ctx, tconn) })
}

// TapHandwritingInputAndWaitForEngineAction returns a uiauto.Action which calls TapHandwritingInputAndWaitForEngine.
func TapHandwritingInputAndWaitForEngineAction(tconn *chrome.TestConn) uiauto.Action {
	return uiauto.NamedAction(
		"vkb.TapHandwritingInputAndWaitForEngineAction(tconn *chrome.TestConn) with )",
		func(ctx context.Context) error { return TapHandwritingInputAndWaitForEngine(ctx, tconn) })
}

// SelectFromSuggestionAction returns a uiauto.Action which calls SelectFromSuggestion.
func SelectFromSuggestionAction(tconn *chrome.TestConn, candidateText string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkb.SelectFromSuggestionAction(tconn *chrome.TestConn, candidateText string) with candidateText=%v)", candidateText),
		func(ctx context.Context) error { return SelectFromSuggestion(ctx, tconn, candidateText) })
}
