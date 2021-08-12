// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAutoShift,
		Desc:         "Checks that auto shift feature of virtual keyboard",
		Contacts:     []string{"shengjun@chromium.org", "tranbaoduy@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		// Auto-shift is primarily designed for tablet mode.
		Pre:          pre.VKEnabledTabletReset,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      5 * time.Minute,
	})
}

func VirtualKeyboardAutoShift(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	ui := uiauto.New(tconn)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	vkbCtx := vkb.NewContext(cr, tconn)

	manualShift := func() uiauto.Action {
		leftShiftKey := nodewith.Name("shift").Ancestor(vkb.NodeFinder.HasClass("key_pos_shift_left"))
		return vkbCtx.TapNode(leftShiftKey)
	}

	shiftLock := func() uiauto.Action {
		return uiauto.Combine("double tap to lock shift state",
			// Throws out error if VK is shifted already.
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			manualShift(),
			manualShift(),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateLocked),
		)
	}

	// Test steps:
	//     -- Check inital state should be unshifted.
	//     -- Manual shift.
	//     -- Manual unshift.
	//     -- Shift lock.
	//     -- Manual unshift.
	validateManualShiftAndShiftLock := func() uiauto.Action {
		return uiauto.Combine("validate manual shift and shift-lock",
			// This scenario should start in unshifted state.
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			// VK should be shifted after manual shift.
			manualShift(),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
			// Sleep 1s to avoid double shift.
			ui.Sleep(time.Second),
			// VK should be unshifted after second manual shift.
			manualShift(),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			ui.Sleep(time.Second),
			// Validate shift-lock.
			shiftLock(),
			// VK should be unshifted after second manual shift.
			manualShift(),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
		)
	}

	// setup makes sure the VK is in unshifted state.
	// It force show VK and manual unshift if VK is shifted before test.
	setup := func(ctx context.Context) error {
		if shiftState, err := vkbCtx.ShiftState(ctx); err != nil {
			return errors.Wrap(err, "failed to get VK shift state in setup")
		} else if shiftState != vkb.ShiftStateNone {
			testing.ContextLog(ctx, "VK remains shifted in last test. Try to manually unshift")
			if err := uiauto.Combine("manually unshift in setup",
				// It does not nothing if VK is already on screen.
				vkbCtx.ShowVirtualKeyboard(),
				manualShift(),
				vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to unshift VK in setup")
			}
		}

		// Making sure VK is hidden. It does not nothing if VK is not on screen.
		return vkbCtx.HideVirtualKeyboard()(ctx)
	}

	// teardown dumps information on errors.
	// Reset VK visibility & shift state is done is done in setup.
	teardown := func(ctx context.Context, subtestName string, hasError func() bool) {
		outDir := filepath.Join(s.OutDir(), subtestName)
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, hasError, cr, subtestName)
	}

	// Test scenarios:
	//     -- VK is auto shifted in empty field.
	//     -- VK is unshifted after first key input.
	//     -- VK is not shifted after space.
	//     -- VK should be shifted in new sentence.
	//     -- VK can be manually shifted and unshifted.
	//     -- VK can be shift-locked.
	validateVKShiftInSentenceMode := func(ctx context.Context, inputField testserver.InputField) error {
		return uiauto.Combine("validate VK shift in sentence mode",
			its.Clear(inputField),
			// VK should be auto shifted in empty field.
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
			vkbCtx.TapKey("H"),
			// VK should be reverted to normal after first type.
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			vkbCtx.TapKeys(strings.Split("ello", "")),
			// VK should not be auto shifted after space
			vkbCtx.TapKeyIgnoringCase("SPACE"),
			vkbCtx.TapKeys(strings.Split("world.", "")),
			// VK should be auto shifted after a full stop and SPACE.
			vkbCtx.TapKeyIgnoringCase("SPACE"),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
			vkbCtx.TapKey("H"),
			// VK should be reverted to normal after first type.
			validateManualShiftAndShiftLock(),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Hello world. H"),
		)(ctx)
	}

	// Test scenarios:
	//     -- VK is auto shifted in empty field.
	//     -- VK is unshifted after first key input.
	//     -- VK is shifted again after space.
	//     -- VK should be shifted in new sentence.
	//     -- VK can be manually shifted and unshifted.
	//     -- VK can be shift-locked.
	validateVKShiftInWordMode := func(ctx context.Context, inputField testserver.InputField) error {
		return uiauto.Combine("validate VK shift in word mode",
			its.Clear(inputField),
			// VK should be auto shifted in empty field.
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
			vkbCtx.TapKey("H"),
			// VK should be reverted to normal after first type.
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			vkbCtx.TapKeys(strings.Split("ello", "")),
			// VK should be auto shifted after space
			vkbCtx.TapKeyIgnoringCase("space"),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
			// VK should be reverted to normal after first type.
			vkbCtx.TapKey("W"),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			vkbCtx.TapKeys(strings.Split("orld.", "")),
			// VK should be auto shifted after a full stop and SPACE.
			vkbCtx.TapKeyIgnoringCase("space"),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateShifted),
			vkbCtx.TapKey("H"),
			// VK should be reverted to normal after first type.
			validateManualShiftAndShiftLock(),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "Hello World. H"),
		)(ctx)
	}

	// Test scenarios:
	//     -- VK is shift-locked in field.
	//     -- VK is not unshifted during typing sentence.
	//     -- VK is not unshifted after changing line.
	validateVKShiftInCharMode := func(ctx context.Context, inputField testserver.InputField) error {
		return uiauto.Combine("validate VK shift in charactor mode",
			its.Clear(inputField),
			// VK should be shift lock in charactor mode.
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateLocked),
			vkbCtx.TapKeys(strings.Split("HELLO", "")),
			vkbCtx.TapKeyIgnoringCase("space"),
			vkbCtx.TapKeys(strings.Split("WORLD", "")),
			// VK should be still shifted in new line.
			vkbCtx.TapKeyIgnoringCase("enter"),
			// VK should not be reverted to normal after first type.
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateLocked),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "HELLO WORLD\n"),
		)(ctx)
	}

	// Test scenarios:
	//     -- VK is not shifted in empty field.
	//     -- VK is not shifted in either new word or new sentence.
	//     -- VK can be manually shifted and unshifted.
	//     -- VK can be shift-locked.
	validateNoVKShift := func(ctx context.Context, inputField testserver.InputField) error {
		return uiauto.Combine("validate VK shift in charactor mode",
			its.Clear(inputField),
			// VK should be shift lock in charactor mode.
			its.ClickFieldUntilVKShown(inputField),
			vkbCtx.WaitUntilShiftStatus(vkb.ShiftStateNone),
			vkbCtx.TapKeys(strings.Split("hello", "")),
			vkbCtx.TapKeyIgnoringCase("space"),
			vkbCtx.TapKeys(strings.Split("world.", "")),
			// VK should not be shifted in next sentence.
			vkbCtx.TapKeyIgnoringCase("space"),
			vkbCtx.TapKey("h"),
			validateManualShiftAndShiftLock(),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "hello world. h"),
		)(ctx)
	}

	runSubtest := func(ctx context.Context, name string, f func(ctx context.Context) error) {
		s.Run(ctx, name, func(context.Context, *testing.State) {
			if err := setup(ctx); err != nil {
				s.Fatal("Failed to setup: ", err)
			}
			defer teardown(ctx, name, s.HasError)
			if err := f(ctx); err != nil {
				s.Fatal("Subtest failed: ", err)
			}
		})
	}

	runSubtest(ctx, "no_attribute", func(ctx context.Context) error {
		return validateVKShiftInSentenceMode(ctx, testserver.TextAreaInputField)
	})

	runSubtest(ctx, "sentence", func(ctx context.Context) error {
		return validateVKShiftInSentenceMode(ctx, testserver.TextAreaAutoShiftInSentence)
	})

	runSubtest(ctx, "word", func(ctx context.Context) error {
		return validateVKShiftInWordMode(ctx, testserver.TextAreaAutoShiftInWord)
	})

	runSubtest(ctx, "char", func(ctx context.Context) error {
		return validateVKShiftInCharMode(ctx, testserver.TextAreaAutoShiftInChar)
	})

	runSubtest(ctx, "off", func(ctx context.Context) error {
		return validateNoVKShift(ctx, testserver.TextAreaAutoShiftOff)
	})

	runSubtest(ctx, "inapplicable", func(ctx context.Context) error {
		return validateNoVKShift(ctx, testserver.URLInputField)
	})
}
