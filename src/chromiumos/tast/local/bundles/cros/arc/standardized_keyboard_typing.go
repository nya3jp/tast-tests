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

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedKeyboardTyping,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests standard keyboard typing functionality. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedKeyboardTypingTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedKeyboardTypingTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedKeyboardTypingTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedKeyboardTypingTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}},
	})
}

// StandardizedKeyboardTyping runs all the provided test cases.
func StandardizedKeyboardTyping(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".TypingTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
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
