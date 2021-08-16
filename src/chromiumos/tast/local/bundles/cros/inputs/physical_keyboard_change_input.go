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

	defaultInputMethod := ime.EnglishUS
	newInputMethod := ime.Japanese

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Fail to launch inputs test server: ", err)
	}
	defer its.Close()

	waitUntilCurrentInputMethod := func(im ime.InputMethod) uiauto.Action {
		return func(ctx context.Context) error {
			fullyQualifiedIMEID, err := im.FullyQualifiedIMEID(ctx, tconn)
			if err != nil {
				return errors.Wrapf(err, "failed to get fully qualified IME ID of %q", im)
			}
			return ime.WaitForInputMethodMatches(ctx, tconn, fullyQualifiedIMEID, 20*time.Second)
		}
	}

	// TODO(b/196771467) Validate typing after switching IME.
	if err := uiauto.Combine("switch IME with shortcut",
		newInputMethod.Install(tconn),
		// Ctrl + Shift + Space changes IME in rotation.
		keyboard.AccelAction("Ctrl+Shift+Space"),
		waitUntilCurrentInputMethod(newInputMethod),
		keyboard.AccelAction("Ctrl+Shift+Space"),
		waitUntilCurrentInputMethod(defaultInputMethod),

		// Ctrl + Space changes to the most recent IME.
		keyboard.AccelAction("Ctrl+Space"),
		waitUntilCurrentInputMethod(newInputMethod),
		keyboard.AccelAction("Ctrl+Space"),
		waitUntilCurrentInputMethod(defaultInputMethod),
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
