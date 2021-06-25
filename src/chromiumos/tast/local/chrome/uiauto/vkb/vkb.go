// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkb contains shared code to interact with the virtual keyboard.
package vkb

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// VirtualKeyboardContext represents a context of virtual keyboard.
type VirtualKeyboardContext struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
	cr    *chrome.Chrome
}

// NewContext creates a new context of virtual keyboard.
func NewContext(cr *chrome.Chrome, tconn *chrome.TestConn) *VirtualKeyboardContext {
	return &VirtualKeyboardContext{
		ui:    uiauto.New(tconn),
		tconn: tconn,
		cr:    cr,
	}
}

// Finder of virtual keyboard root node.
var vkRootFinder = nodewith.Role(role.RootWebArea).Name("Chrome OS Virtual Keyboard")

// NodeFinder returns a finder of node on virtual keyboard.
var NodeFinder = nodewith.Ancestor(vkRootFinder)

// KeyFinder returns a finder of keys on virtual keyboard.
var KeyFinder = NodeFinder.Role(role.Button)

// KeyByNameIgnoringCase returns a virtual keyboard Key button finder with the name ignoring case.
func KeyByNameIgnoringCase(keyName string) *nodewith.Finder {
	return KeyFinder.NameRegex(regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(keyName) + `$`))
}

// UIConn returns a connection to the virtual keyboard HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The connection is lazily created, and this function will block until the
// extension is loaded or ctx's deadline is reached. The caller should close
// the returned connection.
func (vkbCtx *VirtualKeyboardContext) UIConn(ctx context.Context) (*chrome.Conn, error) {
	const extURLPrefix = "chrome-extension://jkghodnilhceideoidjikpgommlajknk/inputview.html"
	f := func(t *target.Info) bool { return strings.HasPrefix(t.URL, extURLPrefix) }
	return vkbCtx.cr.NewConnForTarget(ctx, f)
}

// BackgroundConn returns a connection to the virtual keyboard background page,
// where JavaScript can be executed to simulate interactions with IME.
func (vkbCtx *VirtualKeyboardContext) BackgroundConn(ctx context.Context) (*chrome.Conn, error) {
	const bgPageURLPrefix = "chrome-extension://jkghodnilhceideoidjikpgommlajknk/background"
	bgTargetFilter := func(t *driver.Target) bool {
		return strings.HasPrefix(t.URL, bgPageURLPrefix)
	}
	// Background target from login persists for a few seconds, causing 2 background targets.
	// Polling until connected to the unique target.
	var bconn *chrome.Conn
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		bconn, err = vkbCtx.cr.NewConnForTarget(ctx, bgTargetFilter)
		return err
	}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 3 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for unique virtual keyboard background target")
	}

	return bconn, nil
}

// ShowVirtualKeyboard returns an action forcing the virtual keyboard show up via Chrome API.
// It is not recommended to use on testing a real user input through the virtual keyboard.
// Virtual keyboard should be normally triggered by focusing an input field.
// Usage: It can be used to test Layout and UI interaction in a quick way.
// For example, testing switch layout.
func (vkbCtx *VirtualKeyboardContext) ShowVirtualKeyboard() uiauto.Action {
	return uiauto.Retry(3,
		uiauto.Combine("force show virtual keyboard via Chrome API",
			func(ctx context.Context) error {
				return vkbCtx.tconn.Eval(ctx, `tast.promisify(chrome.inputMethodPrivate.showInputView)()`, nil)
			},
			vkbCtx.WaitLocationStable()))
}

// HideVirtualKeyboard returns an action forcing the virtual keyboard to be hidden via Chrome API.
// It is not recommended to use on testing a real user input through the virtual keyboard.
// Virtual keyboard should be normally triggered by defocusing an input field.
// Usage: It can be used in test cleanup.
func (vkbCtx *VirtualKeyboardContext) HideVirtualKeyboard() uiauto.Action {
	return uiauto.Retry(3,
		uiauto.Combine("force hide virtual keyboard via Chrome API",
			func(ctx context.Context) error {
				return vkbCtx.tconn.Eval(ctx, `tast.promisify(chrome.inputMethodPrivate.hideInputView)()`, nil)
			},
			vkbCtx.WaitUntilHidden()))
}

// IsShown immediately checks whether the virtual keyboard is shown.
// TODO (b/182408845) re-implement the function in case an error happens.
func (vkbCtx *VirtualKeyboardContext) IsShown(ctx context.Context) (bool, error) {
	return vkbCtx.ui.IsNodeFound(ctx, vkRootFinder)
}

// IsKeyShown immediately checks whether the given key is shown.
// TODO (b/182408845) re-implement the function in case an error happens.
func (vkbCtx *VirtualKeyboardContext) IsKeyShown(ctx context.Context, keyName string) (bool, error) {
	return vkbCtx.ui.IsNodeFound(ctx, KeyFinder.Name(keyName))
}

// WaitLocationStable returns an action
// waiting for the virtual keyboard to appear and stable.
func (vkbCtx *VirtualKeyboardContext) WaitLocationStable() uiauto.Action {
	return vkbCtx.ui.WithTimeout(5 * time.Second).WaitForLocation(vkRootFinder)
}

// Location returns stable location of the virtual keyboard.
func (vkbCtx *VirtualKeyboardContext) Location(ctx context.Context) (*coords.Rect, error) {
	return vkbCtx.ui.Location(ctx, vkRootFinder)
}

// WaitUntilHidden returns an action waiting for the virtual keyboard to hide.
// It waits until the node is gone from a11y tree.
func (vkbCtx *VirtualKeyboardContext) WaitUntilHidden() uiauto.Action {
	return vkbCtx.ui.EnsureGoneFor(vkRootFinder, 3*time.Second)
}

// TapKey returns an action simulating a mouse click event on the middle of the specified key via a touch event.
// The key name is case sensitive. It can be any letter of the alphabet, "space" or "backspace".
func (vkbCtx *VirtualKeyboardContext) TapKey(keyName string) uiauto.Action {
	return vkbCtx.tapKeyFunc(keyName, false)
}

// TapKeyIgnoringCase returns an action simulating a mouse click event on the middle of the specified key via a touch event.
// The key name can either be case sensitive or not. It can be any letter of the alphabet, "space" or "backspace".
func (vkbCtx *VirtualKeyboardContext) TapKeyIgnoringCase(keyName string) uiauto.Action {
	return vkbCtx.tapKeyFunc(keyName, true)
}

func (vkbCtx *VirtualKeyboardContext) tapKeyFunc(keyName string, ignoreCase bool) uiauto.Action {
	// Note: Must use mouse Move + Press + Sleep + Release here instead of Click.
	// Mouse click is simulated by calling Chrome private api `chrome.autotestPrivate.mouseClick`.
	// It works for most cases except virtual keyboard.
	// In vkb extension, it listens to keyPress to send vk layout event to decoder
	// before sending the actual key tap event.
	// Mouse click is too quick and causes a racing issue that decoder receives tap key without layout info.
	keyFinder := KeyFinder.Name(keyName)
	if ignoreCase {
		keyFinder = KeyByNameIgnoringCase(keyName)
	}

	return uiauto.Combine("move mouse to key center point and click",
		vkbCtx.ui.MouseMoveTo(keyFinder, 10*time.Millisecond),
		mouse.Press(vkbCtx.tconn, mouse.LeftButton),
		vkbCtx.ui.Sleep(50*time.Millisecond),
		mouse.Release(vkbCtx.tconn, mouse.LeftButton),
	)
}

// TapKeys return an action simulating tap events in the middle of the specified sequence of keys via touch event.
// Each key can be any letter of the alphabet, "space" or "backspace".
// Keys are case sensitive.
func (vkbCtx *VirtualKeyboardContext) TapKeys(keys []string) uiauto.Action {
	return vkbCtx.tapKeysFunc(keys, false)
}

// TapKeysIgnoringCase return an action simulating tap events in the middle of the specified sequence of keys via touch event.
// Each key can be any letter of the alphabet, "space" or "backspace".
// Keys are case insensitive.
func (vkbCtx *VirtualKeyboardContext) TapKeysIgnoringCase(keys []string) uiauto.Action {
	return vkbCtx.tapKeysFunc(keys, true)
}

func (vkbCtx *VirtualKeyboardContext) tapKeysFunc(keys []string, ignoreCase bool) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkbCtx.TapKeys(keys []string) with keys=%v", keys),
		func(ctx context.Context) error {
			for _, key := range keys {
				if err := vkbCtx.tapKeyFunc(key, ignoreCase)(ctx); err != nil {
					return err
				}
				if err := testing.Sleep(ctx, 100*time.Millisecond); err != nil {
					return errors.New("failed to sleep between taping keys")
				}
			}
			return nil
		})
}

// TapKeyJS returns an action simulating a tap event on the middle of the specified key via javascript. The key can
// be any letter of the alphabet, "space" or "backspace".
func (vkbCtx *VirtualKeyboardContext) TapKeyJS(key string) uiauto.Action {
	return func(ctx context.Context) error {
		kconn, err := vkbCtx.UIConn(ctx)
		if err != nil {
			return err
		}
		defer kconn.Close()
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
}

// TapKeysJS returns an action simulating tap events on the middle of the specified sequence of keys via javascript.
// Each keys can be any letter of the alphabet, "space" or "backspace".
func (vkbCtx *VirtualKeyboardContext) TapKeysJS(keys []string) uiauto.Action {
	return uiauto.NamedAction(
		fmt.Sprintf("vkbCtx.TapKeysJS(keys []string) with keys=%v", keys),
		func(ctx context.Context) error {
			for _, key := range keys {
				if err := vkbCtx.TapKeyJS(key)(ctx); err != nil {
					return err
				}
				testing.Sleep(ctx, 100*time.Millisecond)
			}
			return nil
		})
}

// SetFloatingMode returns an action changing the virtual keyboard to floating/dock layout via private javascript function.
func (vkbCtx *VirtualKeyboardContext) SetFloatingMode(enable bool) uiauto.Action {
	flipButtonFinder := KeyFinder.Name("make virtual keyboard movable")
	if !enable {
		flipButtonFinder = KeyFinder.Name("dock virtual keyboard")
	}
	return vkbCtx.ui.LeftClick(flipButtonFinder)
}

// TapKeyboardLayout returns an action clicking keyboard layout to switch.
// The key name is 'Back' in A11y tree.
func (vkbCtx *VirtualKeyboardContext) TapKeyboardLayout() uiauto.Action {
	return vkbCtx.ui.LeftClick(KeyFinder.Name("Back"))
}

// TapAccessPoints returns an action clicking access points button to switch the suggestion bar to layout icons.
func (vkbCtx *VirtualKeyboardContext) TapAccessPoints() uiauto.Action {
	return vkbCtx.ui.LeftClick(KeyFinder.Name("Show access points"))
}

// WaitForKeysExist returns an action waiting for a list of keys to appear on virtual keyboard.
// Note: Should not use FindKeyNode in a loop to implement this function, because it waits for each key within a timeout.
func (vkbCtx *VirtualKeyboardContext) WaitForKeysExist(keys []string) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			var notFoundKeys []string
			for _, key := range keys {
				keyShown, err := vkbCtx.IsKeyShown(ctx, key)
				if err != nil {
					return err
				}
				if !keyShown {
					notFoundKeys = append(notFoundKeys, key)
				}
			}
			if len(notFoundKeys) > 0 {
				return errors.Errorf("these keys are not found: %v", notFoundKeys)
			}
			return nil
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 5 * time.Second})
	}
}

// GetSuggestions returns suggestions that are currently displayed by the
// virtual keyboard.
func (vkbCtx *VirtualKeyboardContext) GetSuggestions(ctx context.Context) ([]string, error) {
	var suggestions []string

	kconn, err := vkbCtx.UIConn(ctx)
	if err != nil {
		return suggestions, err
	}
	defer kconn.Close()

	err = kconn.Eval(ctx, `
	(() => {
		const elems = document.querySelectorAll('.candidate-span');
		return Array.prototype.map.call(elems, x => x.textContent);
	})()
`, &suggestions)
	return suggestions, err
}

// WaitForDecoderEnabled returns an action waiting for decoder to be enabled or disabled.
func (vkbCtx *VirtualKeyboardContext) WaitForDecoderEnabled(enabled bool) uiauto.Action {
	// TODO(b/157686038) A better solution to identify decoder status.
	// Decoder works async in returning status to frontend IME and self loading.
	// Using sleep temporarily before a reliable evaluation api provided in cl/339837443.
	return func(ctx context.Context) error {
		return testing.Sleep(ctx, 10*time.Second)
	}
}

// closeInfoDialogue closes a information dialogue if it exists in a handwriting canvas.
func (vkbCtx *VirtualKeyboardContext) closeInfoDialogue(buttonName string) uiauto.Action {
	dialogueCloseButton := KeyFinder.Name(buttonName)
	// Close the information dialogue if it shows.
	return vkbCtx.ui.IfSuccessThen(
		vkbCtx.ui.WithTimeout(time.Second).WaitUntilExists(dialogueCloseButton),
		vkbCtx.ui.LeftClickUntil(dialogueCloseButton, vkbCtx.ui.WithTimeout(500*time.Millisecond).WaitUntilGone(dialogueCloseButton)))
}

// ClickUntilVKShown returns an action retrying left clicks the node until the vk is shown with no error.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// The interval between clicks and the timeout can be specified using testing.PollOptions.
func (vkbCtx *VirtualKeyboardContext) ClickUntilVKShown(nodeFinder *nodewith.Finder) uiauto.Action {
	ac := vkbCtx.ui.WithPollOpts(testing.PollOptions{Interval: 2 * time.Second, Timeout: 10 * time.Second})
	return uiauto.Retry(5, ac.LeftClickUntil(nodeFinder, vkbCtx.WaitLocationStable()))
}

// SwitchToVoiceInput returns an action changing virtual keyboard to voice input layout.
func (vkbCtx *VirtualKeyboardContext) SwitchToVoiceInput() uiauto.Action {
	// Call background API to switch.
	callSwitchAPI := func(ctx context.Context) error {
		bconn, err := vkbCtx.BackgroundConn(ctx)
		if err != nil {
			return err
		}
		if err := bconn.Eval(ctx, `background.getTestOnlyApi().switchToVoiceInput()`, nil); err != nil {
			return errors.Wrap(err, "failed to call switchToVoiceInput()")
		}
		return nil
	}
	// This node indicates if the voice input is active.
	voiceActiveNode := NodeFinder.HasClass("voice-mic-img")
	return uiauto.Combine("tap voice button and close privacy dialogue",
		// Do nothing if it is already in the voice layout.
		vkbCtx.ui.IfSuccessThen(
			vkbCtx.ui.Gone(voiceActiveNode),
			callSwitchAPI,
		),
		vkbCtx.ui.WaitUntilExists(voiceActiveNode),
	)
}

// switchToHandwriting changes to handwriting layout and returns a handwriting context.
func (vkbCtx *VirtualKeyboardContext) switchToHandwriting(ctx context.Context) (*HandwritingContext, error) {
	if err := vkbCtx.ui.LeftClick(KeyFinder.NameStartingWith("switch to handwriting"))(ctx); err != nil {
		return nil, err
	}

	return vkbCtx.NewHandwritingContext(ctx)
}

// SwitchToHandwritingAndCloseInfoDialogue changes to handwriting layout,
// closes the information dialogue if one shows up and returns a handwriting context.
func (vkbCtx *VirtualKeyboardContext) SwitchToHandwritingAndCloseInfoDialogue(ctx context.Context) (*HandwritingContext, error) {
	hwCtx, err := vkbCtx.switchToHandwriting(ctx)
	if err != nil {
		return nil, err
	}
	// Close info dialogue if one shows up in longform VK.
	if hwCtx.isLongForm {
		if err := hwCtx.closeInfoDialogue("Got it")(ctx); err != nil {
			return nil, err
		}
	}

	return hwCtx, nil
}

// SwitchToMultipaste returns an action changing to multipaste layout.
func (vkbCtx *VirtualKeyboardContext) SwitchToMultipaste() uiauto.Action {
	return vkbCtx.ui.LeftClick(KeyFinder.Name("Multipaste clipboard"))
}

// TapMultipasteItem returns an action tapping the item corresponding to itemName in multipaste virtual keyboard.
func (vkbCtx *VirtualKeyboardContext) TapMultipasteItem(itemName string) uiauto.Action {
	return vkbCtx.ui.LeftClick(NodeFinder.ClassName("scrim").Name(itemName))
}

// DeleteMultipasteItem returns an action selecting a multipaste item via longpress and deleting it.
func (vkbCtx *VirtualKeyboardContext) DeleteMultipasteItem(touchCtx *touch.Context, itemName string) uiauto.Action {
	itemFinder := NodeFinder.ClassName("scrim").Name(itemName)
	return uiauto.Combine("Delete item in multipaste virtual keyboard",
		touchCtx.LongPress(itemFinder),
		vkbCtx.ui.LeftClick(KeyFinder.ClassName("trash-button")),
		vkbCtx.ui.WithTimeout(3*time.Second).WaitUntilGone(itemFinder))
}

// EnableA11yVirtualKeyboard returns an action enabling or disabling
// accessibility mode of the virtual keyboard.
// When disabled, the tablet non-a11y virtual keyboard will be used.
func (vkbCtx *VirtualKeyboardContext) EnableA11yVirtualKeyboard(enabled bool) uiauto.Action {
	return func(ctx context.Context) error {
		return vkbCtx.tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.a11y.virtual_keyboard", enabled)
	}
}

// SelectFromSuggestion returns an action waiting for suggestion candidate to appear and clicks it to select.
func (vkbCtx *VirtualKeyboardContext) SelectFromSuggestion(candidateText string) uiauto.Action {
	suggestionFinder := KeyFinder.Name(candidateText).ClassName("sk")
	opts := testing.PollOptions{Timeout: 3 * time.Second, Interval: 500 * time.Millisecond}
	ac := vkbCtx.ui.WithPollOpts(opts)

	return uiauto.Combine("wait for suggestion and select",
		ac.WaitUntilExists(suggestionFinder),
		ac.LeftClick(suggestionFinder))
}
