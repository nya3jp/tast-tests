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
	displayName  string
	key          input.EventCode
	skipOnTablet bool
}

// allTestKeys holds all the keys under test. Must match keyCodesToTest in the corresponding app.
var allTestKeys = []standardizedKeyboardKeyTest{
	{displayName: "KEYS TEST - LEFT ARROW", key: input.KEY_LEFT, skipOnTablet: false},
	{displayName: "KEYS TEST - DOWN ARROW", key: input.KEY_DOWN, skipOnTablet: false},
	{displayName: "KEYS TEST - RIGHT ARROW", key: input.KEY_RIGHT, skipOnTablet: false},
	{displayName: "KEYS TEST - UP ARROW", key: input.KEY_UP, skipOnTablet: false},
	{displayName: "KEYS TEST - TAB", key: input.KEY_TAB, skipOnTablet: false},
	{displayName: "KEYS TEST - ESCAPE", key: input.KEY_ESC, skipOnTablet: false},
	{displayName: "KEYS TEST - ENTER", key: input.KEY_ENTER, skipOnTablet: false},
	{displayName: "KEYS TEST - FORWARD", key: input.KEY_FORWARD, skipOnTablet: false},
	{displayName: "KEYS TEST - BACK", key: input.KEY_BACK, skipOnTablet: true}, // The back button is actually a gesture in tablet mode.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedKeyboard,
		Desc:         "Functional test that installs an app and tests standard keyboard input functionality. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedKeyboardTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedKeyboardTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedKeyboardTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedKeyboardTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}},
	})
}

// StandardizedKeyboard runs all the provided test cases.
func StandardizedKeyboard(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedKeyboardTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedkeyboardtest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runStandardizedKeyboardTests runs all the tests that are part of the
// standardized keyboard input suite.
func runStandardizedKeyboardTests(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual keyboard: ", err)
	}
	defer kbd.Close()

	runStandardizedKeyboardTypingTest(ctx, s, testParameters, kbd)

	runStandardizedKeyboardCopyPasteTest(ctx, s, testParameters, kbd)

	runStandardizedKeyboardKeysTest(ctx, s, testParameters, kbd)
}

// runStandardizedKeyboardTypingTest types into the input field, and ensures the text appears.
// This does not use the virtual, on screen keyboard.
func runStandardizedKeyboardTypingTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams, kbd *input.KeyboardEventWriter) {
	textKeyboardInputID := testParameters.AppPkgName + ":id/textKeyboardInput"
	textKeyboardSelector := testParameters.Device.Object(ui.ID(textKeyboardInputID))
	const textForTest = "abcdEFGH0123!@#$"

	if err := clickInputAndGuaranteeFocus(ctx, textKeyboardSelector); err != nil {
		s.Fatal("Unable to focus the input, info: ", err)
	}

	if err := kbd.Type(ctx, textForTest); err != nil {
		s.Fatalf("Unable to type: %v, info: %v", textForTest, err)
	}

	if err := testParameters.Device.Object(ui.ID(textKeyboardInputID), ui.Text(textForTest)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatalf("Unable to confirm: %v was typed, info: %v", textForTest, err)
	}
}

// runStandardizedKeyboardCopyPasteTest verifies an input with pre-established source text
// exists, runs a Ctrl+a/Ctrl+c to copy the text, pastes it into a destination, and
// verifies it was properly copied. This does not use the virtual, on screen keyboard.
func runStandardizedKeyboardCopyPasteTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams, kbd *input.KeyboardEventWriter) {
	// Setup the selector ids
	textSourceID := testParameters.AppPkgName + ":id/textCopySource"
	textDestinationID := testParameters.AppPkgName + ":id/textCopyDestination"
	const sourceText = "SOURCE_TEXT_TO_COPY"

	if err := clickInputAndGuaranteeFocus(ctx, testParameters.Device.Object(ui.ID(textSourceID), ui.Text(sourceText))); err != nil {
		s.Fatal("Unable to focus the source input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+a"); err != nil {
		s.Fatal("Unable to send ctrl+a to input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+c"); err != nil {
		s.Fatal("Unable to send ctrl+c to input, info: ", err)
	}

	// Verify the destination field exists and paste into it.
	if err := clickInputAndGuaranteeFocus(ctx, testParameters.Device.Object(ui.ID(textDestinationID))); err != nil {
		s.Fatal("Unable to focus the destination input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+v"); err != nil {
		s.Fatal("Unable to send ctrl+v to input, info: ", err)
	}

	if err := testParameters.Device.Object(ui.ID(textDestinationID), ui.Text(sourceText)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatalf("Unable to confirm: %v was pasted into the destination, info: %v", sourceText, err)
	}
}

// runStandardizedKeyboardKeysTest verifies that all the provided keys are handled by
// the android application's layout when it is focused. This ensures they can all be
// handled by android applications.
func runStandardizedKeyboardKeysTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams, kbd *input.KeyboardEventWriter) {
	// Setup the selector ids
	btnStartKeysTestID := testParameters.AppPkgName + ":id/btnStartKeysTest"
	layoutMainID := testParameters.AppPkgName + ":id/layoutMain"

	if err := testParameters.Device.Object(ui.ID(btnStartKeysTestID)).Click(ctx); err != nil {
		s.Fatal("Unable to start the keys test, info: ", err)
	}

	isFocused, err := testParameters.Device.Object(ui.ID(layoutMainID)).IsFocused(ctx)
	if err != nil {
		s.Fatal("Unable to check focus of layout, info: ", err)
	}

	if isFocused == false {
		s.Fatal("Unable to focus the layout, info: ", err)
	}

	for _, curTestKey := range allTestKeys {
		if curTestKey.skipOnTablet == true && testParameters.InTabletMode == true {
			s.Logf("Skipping test for key: %v while in tablet mode", curTestKey.displayName)
			continue
		}

		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			s.Fatalf("Element for key: %v does not exist, info: %v", curTestKey.displayName, err)
		}

		if err := kbd.TypeKey(ctx, curTestKey.key); err != nil {
			s.Fatalf("Unable to send key: %v to app, info: %v", curTestKey.displayName, err)
		}

		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			s.Fatalf("%v element didn't get removed after key press, info: %v", curTestKey.displayName, err)
		}
	}
}

// clickInputAndGuaranteeFocus makes sure an input exists, clicks it, and ensures it is focused.
func clickInputAndGuaranteeFocus(ctx context.Context, selector *ui.Object) error {
	if err := selector.Exists(ctx); err != nil {
		return errors.Wrap(err, "unable to find the input")
	}

	if err := selector.Click(ctx); err != nil {
		return errors.Wrap(err, "unable to click the input")
	}

	isFocused, err := selector.IsFocused(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to check the inputs focus state")
	}

	if isFocused == false {
		return errors.Wrap(err, "unable to focus the input")
	}

	return nil
}
