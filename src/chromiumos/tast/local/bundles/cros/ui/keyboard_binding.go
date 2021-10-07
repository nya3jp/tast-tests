// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardBinding,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Change and disable keyboard key bindings from OS settings",
		Contacts:     []string{"lance.wang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

type keyboardBindingTestResources struct {
	tconn    *chrome.TestConn
	ui       *uiauto.Context
	kb       *input.KeyboardEventWriter
	settings *ossettings.OSSettings
}

// KeyboardBinding verifies keyboard key bindings can be changed properly.
func KeyboardBinding(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// DUTs that cannot detect physical keyboards will be likely to fail the case.
	// Hence, we prohibit virtual keyboards from performing this test case before further notice.
	isPhysicalKbDetected, _, err := input.FindPhysicalKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to detect physical keyboard: ", err)
	}
	if !isPhysicalKbDetected {
		s.Fatal("Failed to detect physical keyboard: no physical keyboard detected")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

	ui := uiauto.New(tconn)

	res := &keyboardBindingTestResources{
		tconn:    tconn,
		ui:       ui,
		kb:       kb,
		settings: settings,
	}

	// Go to `Keyboard` page.
	keyboardLinkNode := nodewith.HasClass("cr-title-text").Name("Keyboard").Role(role.Heading)
	if err := settings.NavigateToPageURL(ctx, cr, "keyboard-overlay", ui.Exists(keyboardLinkNode)); err != nil {
		s.Fatal("Failed to open `Keyboard settings` page: ", err)
	}

	// The key name and function name of `Search`/`Launcher` will display differently across different models,
	// need to obtain them in advance.
	searchKey, searchFunctionVerifier, err := obtainSearchKeyAndFunction(ctx, res)
	if err != nil {
		s.Fatal("Failed to obtain search key and function: ", err)
	}

	for _, k := range []*key{
		searchKey,
		newCtrlKey(),
		newAltKey(),
		newEscapeKey(),
		newBackspaceKey(),
	} {
		for _, f := range []functionVerifier{
			searchFunctionVerifier(res, k.val),
			newCtrlFunctionVerifier(res, k.val),
			newAltFunctionVerifier(res, k.val),
			newCapslockFunctionVerifier(res, k.val),
			newEscapeFunctionVerifier(res, k.val),
			newBackspaceFunctionVerifier(res, k.val),
			newDisableFunctionVerifier(res, k),
		} {
			name := fmt.Sprintf("bind key %q with function %q", k.name, f.getFunctionName())
			success := s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
				cleanupCtx := ctx
				ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
				defer cancel()

				if err := setKeybinding(res, k, f.getFunctionName())(ctx); err != nil {
					s.Fatal("Failed to set keyboard binding: ", err)
				}
				defer k.resetBinding(res)(cleanupCtx)

				if err := f.setup(ctx); err != nil {
					s.Fatal("Failed to setup for binding test: ", err)
				}
				defer f.cleanup(cleanupCtx)

				if err := f.accel(ctx); err != nil {
					s.Fatal("Failed to accel: ", err)
				}

				uiDump := fmt.Sprintf("%s_%s", k.name, f.getFunctionName())
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, uiDump)

				if err := f.verify(ctx); err != nil {
					s.Fatalf("Failed verify key %q is bind with function %q: %v", k.name, f.getFunctionName(), err)
				}
			})

			if !success {
				s.Errorf("Failed to complete test of bind key %q with function %q", k.name, f.getFunctionName())
			}
		}
	}
}

// obtainSearchKeyAndFunction obtains the corresponding key name and function name of `Search`/`Launcher`.
// The key name and function name of `Search`/`Launcher` will display differently across different models.
func obtainSearchKeyAndFunction(ctx context.Context, res *keyboardBindingTestResources) (*key, func(*keyboardBindingTestResources, string) *searchFunctionVerifier, error) {
	options := nodewith.HasClass("md-select").Role(role.PopUpButton)
	infos, err := res.ui.NodesInfo(ctx, options)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get nodes' information of key binding options")
	}

	for _, info := range infos {
		switch info.Name {
		case string(searchKey):
			return newSearchKey(), newSearchFunctionVerifier, nil
		case string(launcherKey):
			return newLauncherKey(), newLauncherFunctionVerifier, nil
		default:
			continue
		}
	}

	return nil, nil, errors.Wrap(err, "failed to identify the key name and function name of `Search`/`Launch`")
}

func setKeybinding(res *keyboardBindingTestResources, k *key, functionName string) uiauto.Action {
	menu := nodewith.Role(role.PopUpButton).HasClass("md-select").Name(string(k.name))
	targetOption := nodewith.Role(role.ListBoxOption).Name(functionName)

	return uiauto.Combine(fmt.Sprintf("set key %q bind with function %q", k.name, functionName),
		res.settings.LeftClickUntil(menu, res.settings.WithTimeout(3*time.Second).WaitUntilExists(targetOption)),
		res.settings.LeftClickUntil(targetOption, res.settings.WithTimeout(3*time.Second).WaitUntilGone(targetOption)),
	)
}

type keyName string

const (
	searchKey    keyName = "Search"
	launcherKey  keyName = "Launcher"
	ctrlKey      keyName = "Ctrl"
	altKey       keyName = "Alt"
	escapeKey    keyName = "Escape"
	backspaceKey keyName = "Backspace"
)

type key struct {
	name keyName
	val  string
}

func (k *key) resetBinding(res *keyboardBindingTestResources) uiauto.Action {
	return setKeybinding(res, k, string(k.name))
}

func newSearchKey() *key    { return &key{searchKey, "search"} }
func newLauncherKey() *key  { return &key{launcherKey, "search"} } // `Launcher` uses same key code as `Search` does.
func newCtrlKey() *key      { return &key{ctrlKey, "ctrl"} }
func newAltKey() *key       { return &key{altKey, "alt"} }
func newEscapeKey() *key    { return &key{escapeKey, "esc"} }
func newBackspaceKey() *key { return &key{backspaceKey, "backspace"} }

type functionVerifier interface {
	setup(ctx context.Context) error
	cleanup(ctx context.Context) error
	accel(ctx context.Context) error
	verify(ctx context.Context) error

	getFunctionName() string
}

type searchFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal  string
	functionName string
}

func newSearchFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *searchFunctionVerifier {
	return &searchFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		functionName:                 "Search",
	}
}
func (v *searchFunctionVerifier) setup(ctx context.Context) error {
	return nil
}
func (v *searchFunctionVerifier) cleanup(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := v.accel(ctx); err != nil {
			return err
		}
		return ash.WaitForLauncherState(ctx, v.tconn, ash.Closed)
	}, &testing.PollOptions{Timeout: time.Minute})
}
func (v *searchFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}
func (v *searchFunctionVerifier) verify(ctx context.Context) error {
	return ash.WaitForLauncherState(ctx, v.tconn, ash.Peeking)
}
func (v *searchFunctionVerifier) getFunctionName() string { return v.functionName }

func newLauncherFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *searchFunctionVerifier {
	v := newSearchFunctionVerifier(res, boundKeyVal)
	v.functionName = "Launcher"
	return v
}

type ctrlFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal  string
	functionName string
}

func newCtrlFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *ctrlFunctionVerifier {
	return &ctrlFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		functionName:                 "Ctrl",
	}
}
func (v *ctrlFunctionVerifier) setup(ctx context.Context) error {
	_, err := filesapp.Launch(ctx, v.tconn)
	if err != nil {
		return err
	}
	return nil
}
func (v *ctrlFunctionVerifier) cleanup(ctx context.Context) error {
	return apps.Close(ctx, v.tconn, apps.Files.ID)
}
func (v *ctrlFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal + "+w")(ctx)
}
func (v *ctrlFunctionVerifier) verify(ctx context.Context) error {
	return v.ui.WaitUntilGone(filesapp.WindowFinder(apps.Files.ID))(ctx)
}
func (v *ctrlFunctionVerifier) getFunctionName() string { return v.functionName }

type altFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal  string
	functionName string
}

func newAltFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *altFunctionVerifier {
	return &altFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		functionName:                 "Alt",
	}
}
func (v *altFunctionVerifier) setup(ctx context.Context) error {
	_, err := filesapp.Launch(ctx, v.tconn)
	if err != nil {
		return err
	}
	return nil
}
func (v *altFunctionVerifier) cleanup(ctx context.Context) error {
	// Ignore error to ensure filesapp can be closed.
	v.kb.AccelReleaseAction(v.boundKeyVal)(ctx)
	return apps.Close(ctx, v.tconn, apps.Files.ID)
}
func (v *altFunctionVerifier) accel(ctx context.Context) error {
	return uiauto.Combine(fmt.Sprintf("press `%s+tab` to trigger window cycle item view", v.boundKeyVal),
		v.kb.AccelPressAction(v.boundKeyVal),
		v.kb.AccelAction("tab"),
	)(ctx)
}
func (v *altFunctionVerifier) verify(ctx context.Context) error {
	windowCycleListNode := nodewith.HasClass("WindowCycleItemView").Name("Settings - Keyboard").Role(role.Window)
	return v.ui.WaitUntilExists(windowCycleListNode)(ctx)
}
func (v *altFunctionVerifier) getFunctionName() string { return v.functionName }

type capslockFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal       string
	functionName      string
	capsLockIndicator *nodewith.Finder
}

func newCapslockFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *capslockFunctionVerifier {
	return &capslockFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		functionName:                 "Caps Lock",
		capsLockIndicator:            nodewith.HasClass("aura::Window").Name("caps lock on").Role(role.Alert),
	}
}
func (v *capslockFunctionVerifier) setup(ctx context.Context) error {
	return nil
}
func (v *capslockFunctionVerifier) cleanup(ctx context.Context) error {
	return v.ui.WithInterval(5*time.Second).RetryUntil(
		v.kb.AccelAction(v.boundKeyVal),
		v.ui.Gone(v.capsLockIndicator),
	)(ctx)
}
func (v *capslockFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}
func (v *capslockFunctionVerifier) verify(ctx context.Context) error {
	return v.ui.WaitUntilExists(v.capsLockIndicator)(ctx)
}
func (v *capslockFunctionVerifier) getFunctionName() string { return v.functionName }

type escapeFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal  string
	functionName string
}

func newEscapeFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *escapeFunctionVerifier {
	return &escapeFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		functionName:                 "Escape",
	}
}
func (v *escapeFunctionVerifier) setup(ctx context.Context) error {
	return v.ui.WithTimeout(time.Minute).LeftClickUntil(
		ossettings.SearchBoxFinder,
		v.ui.WithTimeout(5*time.Second).WaitUntilExists(ossettings.SearchBoxFinder.Focused()),
	)(ctx)
}
func (v *escapeFunctionVerifier) cleanup(ctx context.Context) error {
	return nil
}
func (v *escapeFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}
func (v *escapeFunctionVerifier) verify(ctx context.Context) error {
	return v.ui.WithTimeout(time.Minute).RetryUntil(
		v.accel,
		v.ui.WithTimeout(5*time.Second).WaitUntilExists(ossettings.SearchBoxFinder.State(state.Focused, false)),
	)(ctx)
}
func (v *escapeFunctionVerifier) getFunctionName() string { return v.functionName }

type backspaceFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal  string
	functionName string
	typeWord     string
}

func newBackspaceFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *backspaceFunctionVerifier {
	return &backspaceFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		functionName:                 "Backspace",
		typeWord:                     "OS version?",
	}
}
func (v *backspaceFunctionVerifier) setup(ctx context.Context) error {
	return uiauto.Combine("setup for verify backspace function",
		v.ui.EnsureFocused(ossettings.SearchBoxFinder),
		v.kb.TypeAction(v.typeWord),
	)(ctx)
}
func (v *backspaceFunctionVerifier) cleanup(ctx context.Context) error {
	settings := ossettings.New(v.tconn)
	return settings.ClearSearch()(ctx)
}
func (v *backspaceFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}
func (v *backspaceFunctionVerifier) verify(ctx context.Context) error {
	expectedWord := v.typeWord[:len(v.typeWord)-1]
	expectedNode := nodewith.Name(expectedWord).Role(role.StaticText).FinalAncestor(ossettings.WindowFinder)
	return v.ui.WaitUntilExists(expectedNode)(ctx)
}
func (v *backspaceFunctionVerifier) getFunctionName() string { return v.functionName }

type disableFunctionVerifier struct {
	*keyboardBindingTestResources
	functionName string

	verifier functionVerifier
}

func newDisableFunctionVerifier(res *keyboardBindingTestResources, boundKey *key) *disableFunctionVerifier {
	v := &disableFunctionVerifier{
		keyboardBindingTestResources: res,
		functionName:                 "Disabled",
	}

	switch boundKey.name {
	case searchKey:
		v.verifier = newSearchFunctionVerifier(res, boundKey.val)
	case launcherKey:
		v.verifier = newLauncherFunctionVerifier(res, boundKey.val)
	case ctrlKey:
		v.verifier = newCtrlFunctionVerifier(res, boundKey.val)
	case altKey:
		v.verifier = newAltFunctionVerifier(res, boundKey.val)
	case escapeKey:
		v.verifier = newEscapeFunctionVerifier(res, boundKey.val)
	case backspaceKey:
		v.verifier = newBackspaceFunctionVerifier(res, boundKey.val)
	default:
		v = nil
	}

	return v
}

func (v *disableFunctionVerifier) setup(ctx context.Context) error {
	return v.verifier.setup(ctx)
}
func (v *disableFunctionVerifier) cleanup(ctx context.Context) error {
	return v.verifier.cleanup(ctx)
}
func (v *disableFunctionVerifier) accel(ctx context.Context) error {
	return v.verifier.accel(ctx)
}
func (v *disableFunctionVerifier) verify(ctx context.Context) error {
	err := v.verifier.verify(ctx)
	if err != nil {
		return nil
	}
	return errors.New("function is not disabled")
}
func (v *disableFunctionVerifier) getFunctionName() string { return v.functionName }
