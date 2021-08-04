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

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedKeyboardCopyPaste,
		Desc:         "Functional test that installs an app and tests standard keyboard copy/paste functionality. Test are performed in clamshell and touchview mode. This does not test the virtual, on-screen keyboard",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}},
	})
}

// StandardizedKeyboardCopyPaste runs all the provided test cases.
func StandardizedKeyboardCopyPaste(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedKeyboardTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedkeyboardtest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runStandardizedKeyboardCopyPasteTest verifies an input with pre-established source text
// exists, runs a Ctrl+a/Ctrl+c to copy the text, pastes it into a destination, and
// verifies it was properly copied. This does not use the virtual, on screen keyboard.
func runStandardizedKeyboardCopyPasteTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual keyboard: ", err)
	}
	defer kbd.Close()

	// Setup the selector ids
	textSourceID := testParameters.AppPkgName + ":id/textCopySource"
	textDestinationID := testParameters.AppPkgName + ":id/textCopyDestination"
	const sourceText = "SOURCE_TEXT_TO_COPY"

	if err := standardizedtestutil.ClickInputAndGuaranteeFocus(ctx, testParameters.Device.Object(ui.ID(textSourceID), ui.Text(sourceText))); err != nil {
		s.Fatal("Unable to focus the source input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+a"); err != nil {
		s.Fatal("Unable to send ctrl+a to input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+c"); err != nil {
		s.Fatal("Unable to send ctrl+c to input, info: ", err)
	}

	// Verify the destination field exists and paste into it.
	if err := standardizedtestutil.ClickInputAndGuaranteeFocus(ctx, testParameters.Device.Object(ui.ID(textDestinationID))); err != nil {
		s.Fatal("Unable to focus the destination input, info: ", err)
	}

	if err := kbd.Accel(ctx, "Ctrl+v"); err != nil {
		s.Fatal("Unable to send ctrl+v to input, info: ", err)
	}

	if err := testParameters.Device.Object(ui.ID(textDestinationID), ui.Text(sourceText)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatalf("Unable to confirm: %v was pasted into the destination, info: %v", sourceText, err)
	}
}
