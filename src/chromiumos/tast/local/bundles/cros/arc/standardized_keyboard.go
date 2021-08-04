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
}

// runStandardizedKeyboardTypingTest types into the input field, and ensures the text appears.
// This does not use the virtual, on screen keyboard.
func runStandardizedKeyboardTypingTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams, kbd *input.KeyboardEventWriter) {
	textKeyboardInputID := testParameters.AppPkgName + ":id/textKeyboardInput"
	textKeyboardSelector := testParameters.Device.Object(ui.ID(textKeyboardInputID))
	const textForTest = "abcdEFGH0123!@#$"

	if err := clickInputAndGuranteeFocus(ctx, textKeyboardSelector); err != nil {
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

	if err := clickInputAndGuranteeFocus(ctx, testParameters.Device.Object(ui.ID(textSourceID), ui.Text(sourceText))); err != nil {
		s.Fatal("Unable to focus the source input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+a"); err != nil {
		s.Fatal("Unable to send ctrl+a to input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+c"); err != nil {
		s.Fatal("Unable to send ctrl+c to input, info: ", err)
	}

	// Verify the destination field exists and paste into it.
	if err := clickInputAndGuranteeFocus(ctx, testParameters.Device.Object(ui.ID(textDestinationID))); err != nil {
		s.Fatal("Unable to focus the destination input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+v"); err != nil {
		s.Fatal("Unable to send ctrl+v to input, info: ", err)
	}

	if err := testParameters.Device.Object(ui.ID(textDestinationID), ui.Text(sourceText)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatalf("Unable to confirm: %v was pasted into the destination, info: %v", sourceText, err)
	}
}

// clickInputAndGuranteeFocus makes sure an input exists, clicks it, and ensures it is focused.
func clickInputAndGuranteeFocus(ctx context.Context, selector *ui.Object) error {
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
