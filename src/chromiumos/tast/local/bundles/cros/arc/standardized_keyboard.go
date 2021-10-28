// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// standardizedKeyboardKeyTest represents a key to verify in the keys test.
type standardizedKeyboardKeyTest struct {
	displayName string
	key         keyboardKey
}

// keyboardKey represents a key that can be pressed on the keyboard. The actual
// implementation of a press is abstracted since certain keys (namely in the top row)
// behave differently on a per device basis.
type keyboardKey struct {
	key input.EventCode
}

// Press sends a key press (down, and up) for the created key. If the key does not
// exist on the device, false is returned.
func (eck *keyboardKey) Press(ctx context.Context, topRow *input.TopRowLayout, kbd *input.KeyboardEventWriter) (pressed bool, err error) {
	// Handle top row keys first.
	if eck.key == input.KEY_FORWARD {
		if topRow.BrowserForward == "" {
			return false, nil
		}

		return true, kbd.Accel(ctx, topRow.BrowserForward)
	} else if eck.key == input.KEY_BACK {
		if topRow.BrowserBack == "" {
			return false, nil
		}

		return true, kbd.Accel(ctx, topRow.BrowserBack)
	} else {
		return true, kbd.TypeKey(ctx, eck.key)
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedKeyboard,
		Desc:         "Functional test that installs an app and tests standard keyboard copy/paste functionality. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.TestCaseParamVal{DeviceMode: standardizedtestutil.ClamshellDeviceMode},
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInClamshellMode",
			ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.TestCaseParamVal{DeviceMode: standardizedtestutil.TabletDeviceMode},
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.TestCaseParamVal{DeviceMode: standardizedtestutil.ClamshellDeviceMode},
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInClamshellMode",
			ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.TestCaseParamVal{DeviceMode: standardizedtestutil.TabletDeviceMode},
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
		}},
	})
}

// StandardizedKeyboard runs all the provided test cases.
func StandardizedKeyboard(ctx context.Context, s *testing.State) {
	const (
		apkName                = "ArcStandardizedInputTest.apk"
		appPkgName             = "org.chromium.arc.testapp.arcstandardizedinputtest"
		copyPasteActivityName  = ".CopyPasteTestActivity"
		keysTestActivityName   = ".KeysTestActivity"
		typingTestActivityName = ".TypingTestActivity"
	)

	testCases := s.Param().(standardizedtestutil.TestCaseParamVal)
	standardizedtestutil.RunTestCases2(ctx, s, testCases, apkName, appPkgName, copyPasteActivityName, "Keyboard - Copy Paste", runStandardizedKeyboardCopyPasteTest)
	standardizedtestutil.RunTestCases2(ctx, s, testCases, apkName, appPkgName, keysTestActivityName, "Keyboard - Keys", runStandardizedKeyboardKeysTest)
	standardizedtestutil.RunTestCases2(ctx, s, testCases, apkName, appPkgName, typingTestActivityName, "Keyboard - Typing", runStandardizedKeyboardTypingTest)
}

// runStandardizedKeyboardCopyPasteTest verifies an input with pre-established source text
// exists, runs a Ctrl+a/Ctrl+c to copy the text, pastes it into a destination, and
// verifies it was properly copied. This does not use the virtual, on screen keyboard.
func runStandardizedKeyboardCopyPasteTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to create virtual keyboard")
	}
	defer kbd.Close()

	// Setup the selector ids
	textSourceID := testParameters.AppPkgName + ":id/textCopySource"
	textDestinationID := testParameters.AppPkgName + ":id/textCopyDestination"
	const sourceText = "SOURCE_TEXT_TO_COPY"

	if err := standardizedtestutil.ClickInputAndGuaranteeFocus(ctx, testParameters.Device.Object(ui.ID(textSourceID), ui.Text(sourceText))); err != nil {
		return errors.Wrap(err, "unable to focus the source input")
	}

	if err := kbd.Accel(ctx, "Ctrl+a"); err != nil {
		return errors.Wrap(err, "unable to send ctrl+a to input")
	}

	if err := kbd.Accel(ctx, "Ctrl+c"); err != nil {
		return errors.Wrap(err, "unable to send ctrl+c to input")
	}

	// Verify the destination field exists and paste into it.
	if err := standardizedtestutil.ClickInputAndGuaranteeFocus(ctx, testParameters.Device.Object(ui.ID(textDestinationID))); err != nil {
		return errors.Wrap(err, "unable to focus the destination input")
	}

	if err := kbd.Accel(ctx, "Ctrl+v"); err != nil {
		return errors.Wrap(err, "unable to send ctrl+v to input")
	}

	if err := testParameters.Device.Object(ui.ID(textDestinationID), ui.Text(sourceText)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrapf(err, "unable to confirm: %v was pasted into the destination", sourceText)
	}

	return nil
}

// runStandardizedKeyboardKeysTest verifies that all the provided keys are handled by
// the android application's layout when it is focused. This ensures they can all be
// handled by android applications.
func runStandardizedKeyboardKeysTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create virtual keyboard")
	}
	defer kbd.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kbd)
	if err != nil {
		return errors.Wrap(err, "failed to load the top-row layout")
	}

	// Set up the basic keys to test. Must match keyCodesToTest in the corresponding app.
	var allTestKeys = []standardizedKeyboardKeyTest{
		{displayName: "KEYS TEST - LEFT ARROW", key: keyboardKey{key: input.KEY_LEFT}},
		{displayName: "KEYS TEST - DOWN ARROW", key: keyboardKey{key: input.KEY_DOWN}},
		{displayName: "KEYS TEST - RIGHT ARROW", key: keyboardKey{key: input.KEY_RIGHT}},
		{displayName: "KEYS TEST - UP ARROW", key: keyboardKey{key: input.KEY_UP}},
		{displayName: "KEYS TEST - TAB", key: keyboardKey{key: input.KEY_TAB}},
		{displayName: "KEYS TEST - ESCAPE", key: keyboardKey{key: input.KEY_ESC}},
		{displayName: "KEYS TEST - ENTER", key: keyboardKey{key: input.KEY_ENTER}},
		{displayName: "KEYS TEST - FORWARD", key: keyboardKey{key: input.KEY_FORWARD}},
		{displayName: "KEYS TEST - BACK", key: keyboardKey{key: input.KEY_BACK}},
	}

	// Set up the selector ids.
	layoutID := standardizedtestutil.StandardizedTestLayoutID(testParameters.AppPkgName)

	isFocused, err := testParameters.Device.Object(ui.ID(layoutID)).IsFocused(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check focus of the layout")
	}

	if isFocused == false {
		return errors.Wrap(err, "failed to focus the layout")
	}

	for _, curTestKey := range allTestKeys {
		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			return errors.Wrapf(err, "failed to find %v element key", curTestKey.displayName)
		}

		keyPressed, err := curTestKey.key.Press(ctx, topRow, kbd)
		if err != nil {
			return errors.Wrapf(err, "failed to send %v key", curTestKey.displayName)
		}

		if !keyPressed {
			testing.ContextLogf(ctx, "Key for test %v does not exist on device and was skipped", curTestKey.displayName)
			continue
		}

		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			return errors.Wrapf(err, "failed to wait for the %v element key to be removed", curTestKey.displayName)
		}
	}

	return nil
}

// runStandardizedKeyboardTypingTest types into the input field, and ensures the text appears.
// This does not use the virtual, on screen keyboard.
func runStandardizedKeyboardTypingTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to create virtual keyboard")
	}
	defer kbd.Close()

	textKeyboardInputID := testParameters.AppPkgName + ":id/textKeyboardInput"
	textKeyboardSelector := testParameters.Device.Object(ui.ID(textKeyboardInputID))
	const textForTest = "abcdEFGH0123!@#$"

	if err := standardizedtestutil.ClickInputAndGuaranteeFocus(ctx, textKeyboardSelector); err != nil {
		return errors.Wrap(err, "unable to focus the input")
	}

	if err := kbd.Type(ctx, textForTest); err != nil {
		return errors.Wrapf(err, "unable to type: %v", textForTest)
	}

	if err := testParameters.Device.Object(ui.ID(textKeyboardInputID), ui.Text(textForTest)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrapf(err, "unable to confirm %v was typed", textForTest)
	}

	return nil
}
