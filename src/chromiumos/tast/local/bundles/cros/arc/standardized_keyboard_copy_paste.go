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
			Val:               standardizedtestutil.GetClamshellTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInClamshellMode",
			ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInClamshellMode",
			ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTests(runStandardizedKeyboardCopyPasteTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
		}},
	})
}

// StandardizedKeyboardCopyPaste runs all the provided test cases.
func StandardizedKeyboardCopyPaste(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".CopyPasteTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
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
