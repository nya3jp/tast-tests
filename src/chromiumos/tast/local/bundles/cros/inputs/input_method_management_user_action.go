// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputMethodManagementUserAction,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that user can manage input methods in OS settings",
		Contacts:     []string{"shengjun@chromium.org", "myy@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Pre:               pre.NonVKClamshellReset,
			}, {
				Name:              "informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Pre:               pre.NonVKClamshellReset,
			},
			{
				Name:              "guest",
				ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
				ExtraAttr:         []string{"group:input-tools-upstream"},
				Pre:               pre.NonVKClamshellInGuest,
			}, {
				Name:              "guest_informational",
				ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
				ExtraAttr:         []string{"informational"},
				Pre:               pre.NonVKClamshellInGuest,
			},
		},
	})
}

func InputMethodManagementUserAction(ctx context.Context, s *testing.State) {
	testInputMethod := ime.JapaneseWithUSKeyboard

	cleanupCtx := ctx
	ctx, shortCancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer shortCancel()

	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := imesettings.AddInputMethodInOSSettings(uc, kb, testInputMethod).Run(ctx); err != nil {
		s.Fatalf("Failed to add input method %q in OS settings: %v", testInputMethod.Name, err)
	}

	if err := imesettings.RemoveInputMethodInOSSettings(uc, testInputMethod).Run(ctx); err != nil {
		s.Fatalf("Failed to remove input method %q in OS settings: %v", testInputMethod.Name, err)
	}
}
