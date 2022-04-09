// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test that installs an app and tests standard keyboard keys like arrows, esc, enter, etc. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedKeyboardKeysTest),
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedKeyboardKeysTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedKeyboardKeysTest),
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedKeyboardKeysTest),
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}},
	})
}

// StandardizedKeyboardKeys runs all the provided test cases.
func StandardizedKeyboardKeys(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".KeysTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
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
