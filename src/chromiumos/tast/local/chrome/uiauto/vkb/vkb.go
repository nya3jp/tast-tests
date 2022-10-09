// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkb contains shared code to interact with the virtual keyboard.
package vkb

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/chrome/webutil"
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

// WithTimeout creates a new VKB context with customized timeout.
func (vkbCtx *VirtualKeyboardContext) WithTimeout(timeout time.Duration) *VirtualKeyboardContext {
	return &VirtualKeyboardContext{
		ui:    vkbCtx.ui.WithTimeout(timeout),
		tconn: vkbCtx.tconn,
		cr:    vkbCtx.cr,
	}
}

// localStorageKey defines the key used in virtual keyboard local storage.
type localStorageKey string

const (
	// voicePrivacyInfo key is defined in http://google3/i18n/input/javascript/chos/message/name.ts.
	voicePrivacyInfo localStorageKey = "voice_privacy_info"
	// showLongformEdu key is defined in http://google3/i18n/input/javascript/chos/ui/widget/longform_dialog_view.ts.
	showLongformEdu localStorageKey = "shownLongformEdu"
)

// Finder of virtual keyboard root node.
var vkRootFinder = nodewith.Role(role.RootWebArea).Name("Chrome OS Virtual Keyboard")

// NodeFinder returns a finder of node on virtual keyboard.
// It finds nodes with `offscreen:false` property to avoid
// finding cached offscreen nodes.
var NodeFinder = nodewith.Ancestor(vkRootFinder).Onscreen().Visible()

// DragPointFinder returns the finder of the float VK drag button.
var DragPointFinder = NodeFinder.Role(role.Button).NameContaining("drag to reposition the keyboard")

// KeyFinder returns a finder of keys on virtual keyboard.
var KeyFinder = NodeFinder.Role(role.Button)

// MultipasteItemFinder returns a finder of multipaste item on virtual keyboard.
var MultipasteItemFinder = NodeFinder.HasClass("scrim")

// MultipasteSuggestionFinder returns a finder of multipaste suggestion on virtual keyboard header bar.
var MultipasteSuggestionFinder = NodeFinder.HasClass("chip")

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
	return uiauto.RetrySilently(3,
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
	return uiauto.RetrySilently(3,
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
	return vkbCtx.ui.WithTimeout(3 * time.Second).WaitUntilGone(vkRootFinder)
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
	keyFinder := KeyFinder.Name(keyName)
	if ignoreCase {
		keyFinder = KeyByNameIgnoringCase(keyName)
	}

	return vkbCtx.TapNode(keyFinder)
}

// TapNode returns an action to tap on a node.
// In most cases, TapKey should be primary function for tapping key.
// This function should only be used when a node can not be unique identified by Name.
// TODO(b/196273235): Refactor vkb.TapKey function to distinguish keyboard, suggestion bar, node.
func (vkbCtx *VirtualKeyboardContext) TapNode(finder *nodewith.Finder) uiauto.Action {
	// Note: Must use mouse Move + Press + Sleep + Release here instead of Click.
	// Mouse click is simulated by calling Chrome private api `chrome.autotestPrivate.mouseClick`.
	// It works for most cases except virtual keyboard.
	// In vkb extension, it listens to keyPress to send vk layout event to decoder
	// before sending the actual key tap event.
	// Mouse click is too quick and causes a racing issue that decoder receives tap key without layout info.
	return uiauto.Combine("move mouse to node center point and click",
		vkbCtx.ui.MouseMoveTo(finder, 10*time.Millisecond),
		mouse.Press(vkbCtx.tconn, mouse.LeftButton),
		uiauto.Sleep(50*time.Millisecond),
		mouse.Release(vkbCtx.tconn, mouse.LeftButton),
	)
}

// DoubleTapNode returns an action to double tap on a node.
// Note: DoubleTapNode cannot be replaced by calling TapNode twice.
// vkbCtx.ui.MouseMoveTo function waits for the node location to be stable.
// It can take ~500ms and causing long sleep between 2 clicks.
func (vkbCtx *VirtualKeyboardContext) DoubleTapNode(finder *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("move mouse to node center point and click",
		vkbCtx.ui.MouseMoveTo(finder, 10*time.Millisecond),
		mouse.Press(vkbCtx.tconn, mouse.LeftButton),
		uiauto.Sleep(50*time.Millisecond),
		mouse.Release(vkbCtx.tconn, mouse.LeftButton),
		uiauto.Sleep(50*time.Millisecond),
		mouse.Press(vkbCtx.tconn, mouse.LeftButton),
		uiauto.Sleep(50*time.Millisecond),
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
				if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
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

// ShowAccessPoints returns an action showing the access points panel.
func (vkbCtx *VirtualKeyboardContext) ShowAccessPoints() uiauto.Action {
	return func(ctx context.Context) error {
		if err := vkbCtx.ui.WaitForLocation(NodeFinder.HasClass("keyboard"))(ctx); err != nil {
			return err
		}
		if err := vkbCtx.ui.WithTimeout(time.Second).WaitUntilExists(KeyFinder.Name("Hide access points"))(ctx); err == nil {
			// "err == nil" means the access points panel is shown.
			return nil
		}
		return vkbCtx.ui.LeftClick(KeyFinder.Name("Show access points"))(ctx)
	}
}

// SetFloatingMode returns an action changing the virtual keyboard to floating/dock layout.
func (vkbCtx *VirtualKeyboardContext) SetFloatingMode(uc *useractions.UserContext, enabled bool) uiauto.Action {
	var switchMode uiauto.Action
	var actionName string
	if enabled {
		actionName = "Switch VK to floating mode"
		flipButtonFinder := KeyFinder.Name("make virtual keyboard movable")
		switchMode = uiauto.IfSuccessThen(
			vkbCtx.ui.WithTimeout(5*time.Second).WaitUntilExists(flipButtonFinder),
			// Switching to float VK is lagging (b/223081262).
			// Using long interval to check VK locationed.
			vkbCtx.ui.LeftClickUntil(flipButtonFinder,
				vkbCtx.ui.WithTimeout(10*time.Second).WithInterval(2*time.Second).WaitForLocation(DragPointFinder),
			),
		)
	} else {
		actionName = "Switch VK to dock mode"
		flipButtonFinder := KeyFinder.Name("dock virtual keyboard")
		switchMode = uiauto.IfSuccessThen(
			vkbCtx.ui.WithTimeout(5*time.Second).WaitUntilExists(flipButtonFinder),
			vkbCtx.ui.LeftClickUntil(flipButtonFinder, vkbCtx.ui.WithTimeout(10*time.Second).WaitUntilGone(DragPointFinder)),
		)
	}
	return uiauto.UserAction(
		actionName,
		uiauto.Combine("switch VK mode",
			vkbCtx.ShowAccessPoints(),
			switchMode,
			vkbCtx.WaitLocationStable(),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeatureFloatVK,
			},
			Tags: []useractions.ActionTag{
				useractions.ActionTagEssentialInputs,
			},
			Callback: func(ctx context.Context, actionError error) error {
				if actionError == nil {
					uc.SetAttribute(useractions.AttributeFloatVK, strconv.FormatBool(enabled))
				}
				return nil
			},
		},
	)
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
		}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 15 * time.Second})
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
	return uiauto.IfSuccessThen(
		vkbCtx.ui.WithTimeout(time.Second).WaitUntilExists(dialogueCloseButton),
		vkbCtx.ui.LeftClickUntil(dialogueCloseButton, vkbCtx.ui.WithTimeout(500*time.Millisecond).WaitUntilGone(dialogueCloseButton)))
}

// ClickUntilVKShown returns an action retrying left clicks the node until the vk is shown with no error.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// The interval between clicks and the timeout can be specified using testing.PollOptions.
func (vkbCtx *VirtualKeyboardContext) ClickUntilVKShown(nodeFinder *nodewith.Finder) uiauto.Action {
	ac := vkbCtx.ui.WithPollOpts(testing.PollOptions{Interval: 2 * time.Second, Timeout: 10 * time.Second})
	return uiauto.RetrySilently(5, ac.LeftClickUntil(nodeFinder, vkbCtx.WaitLocationStable()))
}

// SwitchToKeyboard returns an action changing to keyboard layout.
// TODO(b/195366402): Use test api for switching to keyboard/handwriting mode for VK.
func (vkbCtx *VirtualKeyboardContext) SwitchToKeyboard() uiauto.Action {
	showAccessPointsBtn := KeyFinder.Name("Show access points")
	return uiauto.Combine("switch back to keyboard",
		uiauto.IfSuccessThen(
			vkbCtx.ui.WithTimeout(500*time.Millisecond).WaitUntilExists(showAccessPointsBtn),
			vkbCtx.ui.LeftClick(showAccessPointsBtn),
		),
		vkbCtx.ui.LeftClick(KeyFinder.Name("Back")),
	)
}

// SwitchToVoiceInput returns an action changing virtual keyboard to voice input layout.
func (vkbCtx *VirtualKeyboardContext) SwitchToVoiceInput() uiauto.Action {
	// Call background API to switch.
	callSwitchAPI := func(ctx context.Context) error {
		bconn, err := vkbCtx.BackgroundConn(ctx)
		if err != nil {
			return err
		}
		if err := bconn.Call(ctx, nil, `(info) => {
			window.localStorage.setItem(info, 'true');
			background.getTestOnlyApi().switchToVoiceInput();
		  }`, voicePrivacyInfo); err != nil {
			return errors.Wrap(err, "failed to call switchToVoiceInput()")
		}
		return nil
	}
	// This node indicates if the voice input is active.
	voiceActiveNode := NodeFinder.HasClass("voice-mic-img")
	return uiauto.Retry(3, uiauto.Combine("tap voice button and close privacy dialogue",
		callSwitchAPI,
		vkbCtx.ui.WithTimeout(5*time.Second).WaitUntilExists(voiceActiveNode),
	))
}

// SwitchToHandwriting changes to handwriting layout and returns a handwriting context.
func (vkbCtx *VirtualKeyboardContext) SwitchToHandwriting(ctx context.Context) (*HandwritingContext, error) {
	// Set local storage to override the LF first time tutorial prompt.
	// It does not apply to legacy handwriting.
	bconn, err := vkbCtx.BackgroundConn(ctx)
	if err != nil {
		return nil, err
	}

	if err := bconn.Call(ctx, nil, `(info) => {
		window.localStorage.setItem(info, 'true');
	  }`, showLongformEdu); err != nil {
		return nil, errors.Wrap(err, "failed to set local storage")
	}

	if err := vkbCtx.leftClickIfExist(KeyFinder.NameRegex(regexp.MustCompile("(switch to handwriting.*)|(handwriting)")))(ctx); err != nil {
		return nil, err
	}

	if err := vkbCtx.ui.WaitUntilExists(NodeFinder.Role(role.Canvas))(ctx); err != nil {
		return nil, err
	}

	return vkbCtx.NewHandwritingContext(ctx)
}

// SwitchToSymbolNumberLayout returns an action changing to symbol number layout.
func (vkbCtx *VirtualKeyboardContext) SwitchToSymbolNumberLayout() uiauto.Action {
	return vkbCtx.TapKey("switch to symbols")
}

// SwitchToMultipaste returns an action changing to multipaste layout.
func (vkbCtx *VirtualKeyboardContext) SwitchToMultipaste() uiauto.Action {
	return uiauto.Combine("switch to multipaste keyboard",
		vkbCtx.ShowAccessPoints(),
		vkbCtx.ui.LeftClick(KeyFinder.Name("Multipaste clipboard")),
	)
}

// TapMultipasteItem returns an action tapping the item corresponding to itemName in multipaste virtual keyboard.
func (vkbCtx *VirtualKeyboardContext) TapMultipasteItem(itemName string) uiauto.Action {
	return vkbCtx.ui.LeftClick(MultipasteItemFinder.Name(itemName))
}

// DeleteMultipasteItem returns an action selecting a multipaste item via longpress and deleting it.
func (vkbCtx *VirtualKeyboardContext) DeleteMultipasteItem(touchCtx *touch.Context, itemName string) uiauto.Action {
	itemFinder := MultipasteItemFinder.Name(itemName)
	return uiauto.Combine("Delete item in multipaste virtual keyboard",
		touchCtx.LongPress(itemFinder),
		touchCtx.Tap(KeyFinder.HasClass("trash-button")),
		vkbCtx.ui.WithTimeout(3*time.Second).WaitUntilGone(itemFinder))
}

// TapMultipasteSuggestion returns an action tapping the item corresponding to itemName in multipaste suggestion bar.
func (vkbCtx *VirtualKeyboardContext) TapMultipasteSuggestion(itemName string) uiauto.Action {
	return vkbCtx.ui.LeftClick(MultipasteSuggestionFinder.Name(itemName))
}

// EnableA11yVirtualKeyboard returns an action enabling or disabling
// accessibility mode of the virtual keyboard.
// When disabled, the tablet non-a11y virtual keyboard will be used.
func (vkbCtx *VirtualKeyboardContext) EnableA11yVirtualKeyboard(enabled bool) uiauto.Action {
	return func(ctx context.Context) error {
		return vkbCtx.tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setAllowedPref)`, "settings.a11y.virtual_keyboard", enabled)
	}
}

// SelectFromSuggestion returns an action waiting for suggestion candidate (Case Sensitive) to appear and clicks it to select.
func (vkbCtx *VirtualKeyboardContext) SelectFromSuggestion(candidateText string) uiauto.Action {
	return vkbCtx.selectFromSuggestionFunc(candidateText, false)
}

// SelectFromSuggestionIgnoringCase returns an action waiting for suggestion candidate (Case Insensitive) to appear and clicks it to select.
func (vkbCtx *VirtualKeyboardContext) SelectFromSuggestionIgnoringCase(candidateText string) uiauto.Action {
	return vkbCtx.selectFromSuggestionFunc(candidateText, true)
}

func (vkbCtx *VirtualKeyboardContext) selectFromSuggestionFunc(candidateText string, ignoringCase bool) uiauto.Action {
	suggestionFinder := KeyFinder.Name(candidateText).HasClass("sk")
	if ignoringCase {
		suggestionFinder = KeyByNameIgnoringCase(candidateText).HasClass("sk")
	}
	opts := testing.PollOptions{Timeout: 3 * time.Second, Interval: 500 * time.Millisecond}
	ac := vkbCtx.ui.WithPollOpts(opts)

	return uiauto.Combine("wait for suggestion and select",
		ac.WaitUntilExists(suggestionFinder),
		ac.LeftClick(suggestionFinder))
}

// leftClickIfExist returns an action that checks the existence of a node within a short timeout,
// then clicks it if it exists and does nothing if not.
func (vkbCtx *VirtualKeyboardContext) leftClickIfExist(finder *nodewith.Finder) uiauto.Action {
	return uiauto.IfSuccessThenWithLog(
		vkbCtx.ui.WithTimeout(2*time.Second).WaitUntilExists(finder),
		vkbCtx.ui.LeftClick(finder))
}

// ShiftState describes the shift state of the virtual keyboard.
type ShiftState int

// Available virtual keyboard shift state.
// Use ShiftStateUnknown when any errors happen in fetching shift state.
const (
	ShiftStateNone ShiftState = iota
	ShiftStateShifted
	ShiftStateLocked
	ShiftStateUnknown
)

// String returns the key representative string content of the shift state.
func (shiftState ShiftState) String() string {
	switch shiftState {
	case ShiftStateNone:
		return "none"
	case ShiftStateShifted:
		return "shifted"
	case ShiftStateLocked:
		return "shift-locked"
	}
	return "unknown"
}

// ShiftState identifies and returns the current VK shift state using left-shift key 'data-key' attribute.
// Note: It only works on English(US).
// It works even the VK is not on screen.
// ShiftLeft: VK is not shifted.
// ShiftLeft-shift: Vk is Shifted.
// ShiftLeft-shiftlock: Vk is Shift locked.
// TODO(b/196272947): Support other input methods other than English(US).
func (vkbCtx *VirtualKeyboardContext) ShiftState(ctx context.Context) (ShiftState, error) {
	inputViewConn, err := vkbCtx.UIConn(ctx)
	if err != nil {
		return ShiftStateUnknown, errors.Wrap(err, "failed to connect to input view page")
	}
	var shiftLeftKeyAttr string
	expr := fmt.Sprintf(`shadowPiercingQuery(%q).getAttribute("data-key")`, `div.shift-key`)
	if err := webutil.EvalWithShadowPiercer(ctx, inputViewConn, expr, &shiftLeftKeyAttr); err != nil {
		return ShiftStateUnknown, errors.Wrap(err, "failed to get ShiftLeftKey status")
	}

	switch shiftLeftKeyAttr {
	case "ShiftLeft":
		return ShiftStateNone, nil
	case "ShiftLeft-shift":
		return ShiftStateShifted, nil
	case "ShiftLeft-shiftlock":
		return ShiftStateLocked, nil
	}
	return ShiftStateUnknown, errors.Wrapf(err, "VK shift status %q is unknown", shiftLeftKeyAttr)
}

// WaitUntilShiftStatus waits for up to 5s until the expected VK shift state.
// Note: It only works on US-en.
func (vkbCtx *VirtualKeyboardContext) WaitUntilShiftStatus(expectedShiftState ShiftState) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if currentShiftState, err := vkbCtx.ShiftState(ctx); err != nil {
				return errors.Wrap(err, "failed to get current VK shift status")
			} else if currentShiftState != expectedShiftState {
				return errors.Errorf("unexpected VK shift status: got %q, want %q", currentShiftState, expectedShiftState)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second})
	}
}

// GlideTyping returns a user action to simulate glide typing on virtual keyboard.
// It works on both tablet VK and A11y VK.
func (vkbCtx *VirtualKeyboardContext) GlideTyping(keys []string, validateResultFunc uiauto.Action) uiauto.Action {
	return func(ctx context.Context) error {
		if len(keys) < 2 {
			return errors.New("glide typing only works on multiple keys")
		}

		touchCtx, err := touch.New(ctx, vkbCtx.tconn)
		if err != nil {
			return errors.Wrap(err, "fail to get touch screen")
		}
		defer touchCtx.Close()

		ui := uiauto.New(vkbCtx.tconn)

		initKeyLoc, err := ui.Location(ctx, KeyByNameIgnoringCase(keys[0]))
		if err != nil {
			return errors.Wrap(err, "fail to find the location of first key")
		}

		var gestures []uiauto.Action
		for i := 1; i < len(keys); i++ {
			// Perform a swipe in 50ms and stop 200ms on each key.
			gestures = append(gestures, uiauto.Sleep(200*time.Millisecond))
			if keys[i] == keys[i-1] {
				keyLoc, err := ui.Location(ctx, KeyByNameIgnoringCase(keys[i]))
				if err != nil {
					return errors.Wrapf(err, "fail to find the location of key: %q", keys[i])
				}
				gestures = append(gestures, touchCtx.SwipeTo(keyLoc.TopLeft(), 50*time.Millisecond))
			}
			gestures = append(gestures, touchCtx.SwipeToNode(KeyByNameIgnoringCase(keys[i]), 50*time.Millisecond))
		}
		return uiauto.Combine("swipe to glide typing and validate result",
			touchCtx.Swipe(initKeyLoc.CenterPoint(), gestures...),
			validateResultFunc,
		)(ctx)
	}
}
