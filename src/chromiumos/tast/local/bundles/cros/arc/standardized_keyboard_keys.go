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
	key         input.EventCode
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

	// Setup the basic keys to test. Must match keyCodesToTest in the corresponding app.
	var allTestKeys = []standardizedKeyboardKeyTest{
		{displayName: "KEYS TEST - LEFT ARROW", key: input.KEY_LEFT},
		{displayName: "KEYS TEST - DOWN ARROW", key: input.KEY_DOWN},
		{displayName: "KEYS TEST - RIGHT ARROW", key: input.KEY_RIGHT},
		{displayName: "KEYS TEST - UP ARROW", key: input.KEY_UP},
		{displayName: "KEYS TEST - TAB", key: input.KEY_TAB},
		{displayName: "KEYS TEST - ESCAPE", key: input.KEY_ESC},
		{displayName: "KEYS TEST - ENTER", key: input.KEY_ENTER},
	}

	// Add in the available top row keys.
	topRow, err := input.KeyboardTopRowLayout(ctx, kbd)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	if topRow.BrowserBack != "" {
		allTestKeys = append(allTestKeys, standardizedKeyboardKeyTest{displayName: "KEYS TEST - BACK", key: input.KEY_BACK})
	} else {
		s.Log("No back button found on device, skipping")
	}

	if topRow.BrowserForward != "" {
		allTestKeys = append(allTestKeys, standardizedKeyboardKeyTest{displayName: "KEYS TEST - FORWARD", key: input.KEY_FORWARD})
	} else {
		s.Log("No forward button found on device, skipping")
	}

	// Setup the selector ids.
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

		if err := kbd.TypeKey(ctx, curTestKey.key); err != nil {
			s.Fatalf("Failed to send %v key: %v", curTestKey.displayName, err)
		}

		if err := testParameters.Device.Object(ui.Text(curTestKey.displayName)).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			s.Fatalf("Failed to wait for the %v element key to be removed: %v", curTestKey.displayName, err)
		}
	}
}
