// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

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
		Func:         StandardizedKeyboardKeys,
		Desc:         "Functional test that installs an app and tests standard keyboard keys like arrows, esc, enter, etc. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTests(runStandardizedKeyboardKeysTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTests(runStandardizedKeyboardKeysTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTests(runStandardizedKeyboardKeysTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTests(runStandardizedKeyboardKeysTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
		}},
	})
}

// StandardizedKeyboardKeys runs all the provided test cases.
func StandardizedKeyboardKeys(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedKeyboardTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedkeyboardtest"
		activityName = ".KeysTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runStandardizedKeyboardKeysTest verifies that all the provided keys are handled by
// the android application's layout when it is focused. This ensures they can all be
// handled by android applications.
func runStandardizedKeyboardKeysTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams) {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create virtual keyboard: ", err)
	}
	defer kbd.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kbd)
	if err != nil {
		s.Fatal("Failed to load the top-row layout: ", err)
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
	layoutMainID := testParameters.AppPkgName + ":id/layoutMain"

	isFocused, err := testParameters.Device.Object(ui.ID(layoutMainID)).IsFocused(ctx)
	if err != nil {
		s.Fatal("Failed to check focus of the layout: ", err)
	}

	if isFocused == false {
		s.Fatal("Failed to focus the layout: ", err)
	}

	for _, curTestKey := range allTestKeys {
		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			s.Fatalf("Failed to find %v element key: %v", curTestKey.displayName, err)
		}

		keyPressed, err := curTestKey.key.Press(ctx, topRow, kbd)
		if err != nil {
			s.Fatalf("Failed to send %v key: %v", curTestKey.displayName, err)
		}

		if !keyPressed {
			s.Logf("Key for test %v does not exist on device and was skipped", curTestKey.displayName)
			continue
		}

		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			s.Fatalf("Failed to wait for the %v element key to be removed: %v", curTestKey.displayName, err)
		}
	}
}
