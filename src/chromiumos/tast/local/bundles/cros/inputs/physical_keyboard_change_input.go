// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardChangeInput,
		Desc:         "Checks that changing input method in different ways on physical keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Pre:               pre.NonVKClamshellReset,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			Pre:               pre.NonVKClamshellReset,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}},
	})
}

func PhysicalKeyboardChangeInput(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	newInputMethods := []ime.InputMethod{ime.Japanese, ime.ChinesePinyin}

	for _, newInputMethod := range newInputMethods {
		if err := newInputMethod.Install(tconn)(ctx); err != nil {
			s.Fatalf("Failed to install new input method %q: %v", newInputMethod, err)
		}
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// Retrieve all installed input methods via Chrome API.
	// Then parse it into ime.InputMethod struct and append to installedInputMethods list.
	var installedInputMethods []*ime.InputMethod
	if installedBindingInputMethod, err := ime.InstalledInputMethods(ctx, tconn); err != nil {
		s.Fatal("Failed to get installed input methods: ", err)
	} else {
		for _, bindingInputMethod := range installedBindingInputMethod {
			if im, err := ime.FindInputMethodByFullyQualifiedIMEID(ctx, tconn, bindingInputMethod.ID); err != nil {
				s.Fatalf("Failed to recognize input method id %q: %v", bindingInputMethod.ID, err)
			} else {
				installedInputMethods = append(installedInputMethods, im)
			}
		}
	}

	currentInputMethod := ime.EnglishUS
	var prevInputMethod ime.InputMethod

	// expectedNextInputMethod finds the expected next input method in order.
	findNextInputMethod := func() ime.InputMethod {
		for index, im := range installedInputMethods {
			if im.Equal(currentInputMethod) {
				return *installedInputMethods[(index+1)%len(installedInputMethods)]
			}
		}
		s.Fatalf("Failed to find the next input method of %q", currentInputMethod)
		return ime.DefaultInputMethod
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// TODO(b/196771467) Replace this function with im.WaitUntilActivated() once adding typing validation.
	waitUntilCurrentInputMethod := func(im ime.InputMethod) uiauto.Action {
		return func(ctx context.Context) error {
			fullyQualifiedIMEID, err := im.FullyQualifiedIMEID(ctx, tconn)
			if err != nil {
				return errors.Wrapf(err, "failed to get fully qualified IME ID of %q", im)
			}
			return ime.WaitForInputMethodMatches(ctx, tconn, fullyQualifiedIMEID, 20*time.Second)
		}
	}

	switchToNextInputMethod := func(ctx context.Context) error {
		nextInputMethod := findNextInputMethod()
		// Ctrl + Shift + Space switches IME to the next in order.
		if err := uiauto.Combine("switch to next input method",
			keyboard.AccelAction("Ctrl+Shift+Space"),
			waitUntilCurrentInputMethod(nextInputMethod),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch to next input method")
		}
		prevInputMethod = currentInputMethod
		currentInputMethod = nextInputMethod
		return nil
	}

	switchToRecentInputMethod := func(ctx context.Context) error {
		// Ctrl + Space changes to the most recent IME.
		if err := uiauto.Combine("switch to recent input method",
			keyboard.AccelAction("Ctrl+Space"),
			waitUntilCurrentInputMethod(prevInputMethod),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch to recent input method")
		}
		currentInputMethod, prevInputMethod = prevInputMethod, currentInputMethod
		return nil
	}

	// TODO(b/196771467) Validate typing after switching IME.
	if err := uiauto.Combine("switch IME in different ways",
		switchToNextInputMethod,
		switchToNextInputMethod,
		switchToNextInputMethod,
		switchToRecentInputMethod,
		switchToRecentInputMethod,
	)(ctx); err != nil {
		s.Fatal("Failed to switch input method: ", err)
	}

	ui := uiauto.New(tconn)
	capsOnImageFinder := nodewith.Name("CAPS LOCK is on").Role(role.Image)

	if err := uiauto.Combine("caps lock with shortcut",
		// Alt + Search locks caps.
		keyboard.AccelAction("Alt+Search"),
		ui.WaitUntilExists(capsOnImageFinder),
		// Shift to unlock.
		keyboard.AccelAction("Shift"),
		ui.WaitUntilGone(capsOnImageFinder),
	)(ctx); err != nil {
		s.Fatal("Failed to validate caps lock: ", err)
	}
}
