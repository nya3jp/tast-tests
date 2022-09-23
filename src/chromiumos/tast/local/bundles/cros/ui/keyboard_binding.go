// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardBinding,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Change and disable keyboard key bindings from OS settings",
		Contacts:     []string{"lance.wang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Keyboard()),
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

// keyboardBindingTestResources aggregates all resources needed for keyboard binding test.
type keyboardBindingTestResources struct {
	tconn    *chrome.TestConn
	ui       *uiauto.Context
	kb       *input.KeyboardEventWriter
	settings *ossettings.OSSettings
}

// key represents a binding option and its keyboard keycode (aka keys).
type key struct {
	name keyName
	val  string
}

// KeyboardBinding verifies keyboard key bindings can be changed properly.
func KeyboardBinding(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Physical keyboard is required for this test.
	// Some models satisfy "HardwareDeps: hwdep.D(hwdep.Keyboard())" but cannot detect physical keyboards.
	// TODO: Remove this once b/223069313 fixed.
	isPhysicalKbDetected, _, err := input.FindPhysicalKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to detect physical keyboard: ", err)
	}
	if !isPhysicalKbDetected {
		s.Fatal("Failed to find a physical keyboard, this test requires a physical keyboard (b/223069313)")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	if err := screenRecorder.Start(ctx, tconn); err != nil {
		s.Log("Failed to start ScreenRecorder: ", err)
	}
	defer uiauto.ScreenRecorderStopSaveRelease(cleanupCtx, screenRecorder, filepath.Join(s.OutDir(), "record.webm"))

	// Ensure tablet mode is disabled. This test case requires DUT to stay in clamshell mode
	// in order to perform keyboard shortcuts and window operations.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode is disabled: ", err)
	}
	defer cleanup(cleanupCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	res := &keyboardBindingTestResources{
		tconn: tconn,
		ui:    uiauto.New(tconn),
		kb:    kb,
	}

	// Open Settings and go to "Keyboard" page.
	keyboardLinkNode := nodewith.HasClass("cr-title-text").Name("Keyboard").Role(role.Heading)
	res.settings, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, "keyboard-overlay", res.ui.Exists(keyboardLinkNode))
	if err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}
	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		res.settings.Close(ctx)
	}(cleanupCtx)

	// The key name and function name of "Search"/"Launcher" will display differently across different models,
	// need to obtain them in advance.
	searchKey, searchFunctionVerifier, err := obtainSearchKeyAndFunction(ctx, res.ui)
	if err != nil {
		s.Fatal("Failed to obtain search key and function: ", err)
	}
	s.Logf("The Search key name on the DUT is %q", searchKey.name)

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
			name := fmt.Sprintf("bind key %q with function %q", k.name, f.functionName())
			success := s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
				cleanupCtx := ctx
				ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
				defer cancel()

				if err := setKeybinding(res, k, f.functionName())(ctx); err != nil {
					s.Fatal("Failed to set keyboard binding: ", err)
				}
				defer resetBinding(res, k)(cleanupCtx)

				if err := f.setup(ctx); err != nil {
					s.Fatal("Failed to setup for binding test: ", err)
				}
				defer func(ctx context.Context) {
					if err := f.cleanup(ctx); err != nil {
						s.Logf("Failed to cleanup after binding test of verifying key %q is bind with function %q: %v", k.name, f.functionName(), err)
					}
				}(cleanupCtx)

				if err := f.accel(ctx); err != nil {
					s.Fatal("Failed to accel: ", err)
				}

				uiDump := fmt.Sprintf("%s_%s", k.name, f.functionName())
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, uiDump)

				if err := f.verify(ctx); err != nil {
					s.Fatalf("Failed to verify key %q is bind with function %q: %v", k.name, f.functionName(), err)
				}
			})

			if !success {
				s.Errorf("Failed to complete test of bind key %q with function %q", k.name, f.functionName())
			}
		}
	}
}

// obtainSearchKeyAndFunction obtains the corresponding key name and function name of "Search"/"Launcher".
// The key name and function name of "Search"/"Launcher" will display differently across different models.
func obtainSearchKeyAndFunction(ctx context.Context, ui *uiauto.Context) (*key, func(*keyboardBindingTestResources, string) *searchFunctionVerifier, error) {
	nameRegex := regexp.MustCompile(fmt.Sprintf(`^(%s|%s)$`, searchKey, launcherKey))
	option := nodewith.NameRegex(nameRegex).HasClass("md-select").Role(role.ComboBoxSelect)

	if err := ui.WaitUntilExists(option)(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to find key with name 'Search' or 'Launcher'")
	}

	info, err := ui.Info(ctx, option)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get info of key with name 'Search' or 'Launcher'")
	}

	switch info.Name {
	case string(searchKey):
		return newSearchKey(), newSearchFunctionVerifier, nil
	case string(launcherKey):
		return newLauncherKey(), newLauncherFunctionVerifier, nil
	default:
		return nil, nil, errors.Errorf("unexpected name: %q", info.Name)
	}
}

// setKeybinding sets the key binding of the key to the specified option.
func setKeybinding(res *keyboardBindingTestResources, k *key, functionName string) uiauto.Action {
	menu := nodewith.Role(role.ComboBoxSelect).HasClass("md-select").Name(string(k.name))
	targetOption := nodewith.Role(role.ListBoxOption).Name(functionName)

	return uiauto.Combine(fmt.Sprintf("set key %q bind with function %q", k.name, functionName),
		res.settings.LeftClickUntil(menu, res.settings.WithTimeout(3*time.Second).WaitUntilExists(targetOption)),
		res.settings.LeftClickUntil(targetOption, res.settings.WithTimeout(3*time.Second).WaitUntilGone(targetOption)),
	)
}

// resetBinding resets the key binding of the key to its default value.
func resetBinding(res *keyboardBindingTestResources, k *key) uiauto.Action {
	return setKeybinding(res, k, string(k.name))
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

func newSearchKey() *key    { return &key{searchKey, "search"} }
func newLauncherKey() *key  { return &key{launcherKey, "search"} } // "Launcher" uses same key code as "Search" does.
func newCtrlKey() *key      { return &key{ctrlKey, "ctrl"} }
func newAltKey() *key       { return &key{altKey, "alt"} }
func newEscapeKey() *key    { return &key{escapeKey, "esc"} }
func newBackspaceKey() *key { return &key{backspaceKey, "backspace"} }

type functionVerifier interface {
	setup(ctx context.Context) error
	cleanup(ctx context.Context) error
	accel(ctx context.Context) error
	verify(ctx context.Context) error

	functionName() string
}

type searchFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal string
	function    string
}

// newSearchFunctionVerifier returns a functionVerifier for "Search"/"Launcher".
func newSearchFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *searchFunctionVerifier {
	return &searchFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		function:                     "Search",
	}
}

// setup sets up the test environment for "Search"/"Launcher".
func (v *searchFunctionVerifier) setup(ctx context.Context) error {
	return nil
}

// cleanup cleans up the test environment for "Search"/"Launcher".
func (v *searchFunctionVerifier) cleanup(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := v.accel(ctx); err != nil {
			return err
		}
		return ash.WaitForLauncherState(ctx, v.tconn, ash.Closed)
	}, &testing.PollOptions{Timeout: time.Minute})
}

// accel accelerates the key to trigger the function.
func (v *searchFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}

// verify verifies if the key is triggered.
func (v *searchFunctionVerifier) verify(ctx context.Context) error {
	return launcher.WaitForClamshellLauncherSearchExit(v.tconn)(ctx)
}

// functionName returns the function name of "Search"/"Launcher".
func (v *searchFunctionVerifier) functionName() string { return v.function }

// newLauncherFunctionVerifier is a functionVerifier for "Launcher".
func newLauncherFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *searchFunctionVerifier {
	v := newSearchFunctionVerifier(res, boundKeyVal)
	v.function = "Launcher"
	return v
}

type ctrlFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal string
	function    string
}

// newCtrlFunctionVerifier returns a functionVerifier for "Ctrl".
func newCtrlFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *ctrlFunctionVerifier {
	return &ctrlFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		function:                     "Ctrl",
	}
}

// setup sets up the test environment for "Ctrl".
func (v *ctrlFunctionVerifier) setup(ctx context.Context) error {
	_, err := filesapp.Launch(ctx, v.tconn)
	if err != nil {
		return err
	}
	return nil
}

// cleanup cleans up the test environment for "Ctrl".
func (v *ctrlFunctionVerifier) cleanup(ctx context.Context) error {
	return apps.Close(ctx, v.tconn, apps.FilesSWA.ID)
}

// accel accelerates the key to trigger the function.
func (v *ctrlFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal + "+w")(ctx)
}

// verify verifies if the key is triggered.
func (v *ctrlFunctionVerifier) verify(ctx context.Context) error {
	return v.ui.WaitUntilGone(filesapp.WindowFinder(apps.FilesSWA.ID))(ctx)
}

// functionName returns the function name of "Ctrl".
func (v *ctrlFunctionVerifier) functionName() string { return v.function }

type altFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal string
	function    string
}

// newAltFunctionVerifier returns a functionVerifier for "Alt".
func newAltFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *altFunctionVerifier {
	return &altFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		function:                     "Alt",
	}
}

// setup sets up the test environment for "Alt".
func (v *altFunctionVerifier) setup(ctx context.Context) error {
	_, err := filesapp.Launch(ctx, v.tconn)
	if err != nil {
		return err
	}
	return nil
}

// cleanup cleans up the test environment for "Alt".
func (v *altFunctionVerifier) cleanup(ctx context.Context) error {
	// Ignore error to ensure filesapp can be closed.
	v.kb.AccelReleaseAction(v.boundKeyVal)(ctx)
	return apps.Close(ctx, v.tconn, apps.FilesSWA.ID)
}

// accel accelerates the key to trigger the function.
func (v *altFunctionVerifier) accel(ctx context.Context) error {
	return uiauto.Combine(fmt.Sprintf("press '%s+tab' to trigger window cycle item view", v.boundKeyVal),
		v.kb.AccelPressAction(v.boundKeyVal),
		v.kb.AccelAction("tab"),
	)(ctx)
}

// verify verifies if the key is triggered.
func (v *altFunctionVerifier) verify(ctx context.Context) error {
	windowCycleListNode := nodewith.HasClass("WindowCycleItemView").Name("Settings - Keyboard").Role(role.Window)
	return v.ui.WaitUntilExists(windowCycleListNode)(ctx)
}

// functionName returns the function name of "Alt".
func (v *altFunctionVerifier) functionName() string { return v.function }

type capslockFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal       string
	function          string
	capsLockIndicator *nodewith.Finder
}

// newCapslockFunctionVerifier returns a functionVerifier for "Caps Lock".
func newCapslockFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *capslockFunctionVerifier {
	return &capslockFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		function:                     "Caps Lock",
		capsLockIndicator:            nodewith.HasClass("aura::Window").Name("caps lock on").Role(role.Alert),
	}
}

// setup sets up the test environment for "Caps Lock".
func (v *capslockFunctionVerifier) setup(ctx context.Context) error {
	return nil
}

// cleanup cleans up the test environment for "Caps Lock".
func (v *capslockFunctionVerifier) cleanup(ctx context.Context) error {
	return v.ui.WithInterval(5*time.Second).RetryUntil(
		v.kb.AccelAction(v.boundKeyVal),
		v.ui.Gone(v.capsLockIndicator),
	)(ctx)
}

// accel accelerates the key to trigger the function.
func (v *capslockFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}

// verify verifies if the key is triggered.
func (v *capslockFunctionVerifier) verify(ctx context.Context) error {
	return v.ui.WaitUntilExists(v.capsLockIndicator)(ctx)
}

// functionName returns the function name of "Caps Lock".
func (v *capslockFunctionVerifier) functionName() string { return v.function }

type escapeFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal string
	function    string
}

// newEscapeFunctionVerifier returns a functionVerifier for "Escape".
func newEscapeFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *escapeFunctionVerifier {
	return &escapeFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		function:                     "Escape",
	}
}

// setup sets up the test environment for "Escape".
func (v *escapeFunctionVerifier) setup(ctx context.Context) error {
	return v.ui.WithTimeout(time.Minute).LeftClickUntil(
		ossettings.SearchBoxFinder,
		v.ui.WithTimeout(5*time.Second).WaitUntilExists(ossettings.SearchBoxFinder.Focused()),
	)(ctx)
}

// cleanup cleans up the test environment for "Escape".
func (v *escapeFunctionVerifier) cleanup(ctx context.Context) error {
	return nil
}

// accel accelerates the key to trigger the function.
func (v *escapeFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}

// verify verifies if the key is triggered.
func (v *escapeFunctionVerifier) verify(ctx context.Context) error {
	return v.ui.WithTimeout(time.Minute).RetryUntil(
		v.accel,
		v.ui.WithTimeout(5*time.Second).WaitUntilExists(ossettings.SearchBoxFinder.State(state.Focused, false)),
	)(ctx)
}

// functionName returns the function name of "Escape".
func (v *escapeFunctionVerifier) functionName() string { return v.function }

type backspaceFunctionVerifier struct {
	*keyboardBindingTestResources
	boundKeyVal string
	function    string
	typeWord    string
}

// newBackspaceFunctionVerifier returns a functionVerifier for "Backspace".
func newBackspaceFunctionVerifier(res *keyboardBindingTestResources, boundKeyVal string) *backspaceFunctionVerifier {
	return &backspaceFunctionVerifier{
		keyboardBindingTestResources: res,
		boundKeyVal:                  boundKeyVal,
		function:                     "Backspace",
		typeWord:                     "OS version?",
	}
}

// setup sets up the test environment for "Backspace".
func (v *backspaceFunctionVerifier) setup(ctx context.Context) error {
	return uiauto.Combine("setup for verify backspace function",
		v.ui.EnsureFocused(ossettings.SearchBoxFinder),
		v.kb.TypeAction(v.typeWord),
	)(ctx)
}

// cleanup cleans up the test environment for "Backspace".
func (v *backspaceFunctionVerifier) cleanup(ctx context.Context) error {
	settings := ossettings.New(v.tconn)
	return settings.ClearSearch()(ctx)
}

// accel accelerates the key to trigger the function.
func (v *backspaceFunctionVerifier) accel(ctx context.Context) error {
	return v.kb.AccelAction(v.boundKeyVal)(ctx)
}

// verify verifies if the key is triggered.
func (v *backspaceFunctionVerifier) verify(ctx context.Context) error {
	expectedWord := v.typeWord[:len(v.typeWord)-1]
	expectedNode := nodewith.Name(expectedWord).Role(role.StaticText).FinalAncestor(ossettings.WindowFinder)
	return v.ui.WaitUntilExists(expectedNode)(ctx)
}

// functionName returns the function name of "Backspace".
func (v *backspaceFunctionVerifier) functionName() string { return v.function }

type disableFunctionVerifier struct {
	*keyboardBindingTestResources
	function string

	verifier functionVerifier
}

// newDisableFunctionVerifier returns a functionVerifier for "Disable".
func newDisableFunctionVerifier(res *keyboardBindingTestResources, boundKey *key) *disableFunctionVerifier {
	v := &disableFunctionVerifier{
		keyboardBindingTestResources: res,
		function:                     "Disabled",
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

// setup sets up the test environment for "Disable".
func (v *disableFunctionVerifier) setup(ctx context.Context) error {
	return v.verifier.setup(ctx)
}

// cleanup cleans up the test environment for "Disable".
func (v *disableFunctionVerifier) cleanup(ctx context.Context) error {
	return v.verifier.cleanup(ctx)
}

// accel accelerates the key to trigger the function.
func (v *disableFunctionVerifier) accel(ctx context.Context) error {
	return v.verifier.accel(ctx)
}

// verify verifies if the key is triggered.
func (v *disableFunctionVerifier) verify(ctx context.Context) error {
	err := v.verifier.verify(ctx)
	if err != nil {
		return nil
	}
	return errors.New("function is not disabled")
}

// functionName returns the function name of "Disable".
func (v *disableFunctionVerifier) functionName() string { return v.function }
