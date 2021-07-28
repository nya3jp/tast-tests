// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const textForTest = "abcdEFGH0123!@#$"

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedKeyboardTyping,
		Desc:         "Functional test that installs an app, and verifies content can be typed into a text field with the keyboard. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               testutil.GetStandardizedClamshellTests(runKeyboardTypingTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: testutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               testutil.GetStandardizedTabletTests(runKeyboardTypingTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: testutil.GetStandardizedTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               testutil.GetStandardizedClamshellTests(runKeyboardTypingTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: testutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               testutil.GetStandardizedTabletTests(runKeyboardTypingTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: testutil.GetStandardizedTabletHardwareDeps(),
		}},
	})
}

// StandardizedKeyboardTyping runs all the provided test cases.
func StandardizedKeyboardTyping(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcAppCompatStandardizedKeyboardTypingTest.apk"
		appName      = "org.chromium.arc.testapp.arcappcompatstandardizedkeyboardtypingtest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]testutil.StandardizedTestCase)
	testutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runKeyboardTypingTests Loads the android application, types into the input
// field, and ensures the text appears. This does not use the virtual, on screen keyboard.
func runKeyboardTypingTests(ctx context.Context, s *testing.State, testParameters testutil.StandardizedTestFuncParams) {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual keyboard: ", err)
	}
	defer kbd.Close()

	textKeyboardInputID := testParameters.AppPkgName + ":id/textKeyboardInput"
	textKeyboardSelector := testParameters.Device.Object(ui.ID(textKeyboardInputID))

	if err := textKeyboardSelector.Exists(ctx); err != nil {
		s.Fatal("Unable to find the input, info: ", err)
	}

	if err := textKeyboardSelector.Click(ctx); err != nil {
		s.Fatal("Unable to click the input")
	}

	isFocused, err := textKeyboardSelector.IsFocused(ctx)
	if err != nil {
		s.Fatal("Unable to check the inputs focus state, info: ", err)
	}

	if isFocused == false {
		s.Fatal("Input could not be focused")
	}

	if err := kbd.Type(ctx, textForTest); err != nil {
		s.Fatalf("Unable to type: %v, info: %v", textForTest, err)
	}

	if err := testParameters.Device.Object(ui.ID(textKeyboardInputID), ui.Text(textForTest)).WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Fatalf("Unable to confirm: %v was typed, info: %v", textForTest, err)
	}
}
