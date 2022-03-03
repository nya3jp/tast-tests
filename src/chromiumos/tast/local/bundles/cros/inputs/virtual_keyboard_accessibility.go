// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package inputs contains local Tast tests that exercise Chrome OS essential inputs.
package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAccessibility,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks that the accessibility keyboard displays correctly",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{{
			Pre:               pre.VKEnabledClamshell,
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "informational",
			Pre:               pre.VKEnabledClamshell,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func VirtualKeyboardAccessibility(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	vkbCtx := vkb.NewContext(cr, tconn)

	// Check that the keyboard has modifier and tab keys.
	keys := []string{"ctrl", "alt", "caps lock", "tab"}
	action := uiauto.Combine("trigger A11y virtual keyboard and check functional keys exist",
		vkbCtx.ShowVirtualKeyboard(),
		vkbCtx.WaitForKeysExist(keys),
	)

	if err := uiauto.UserAction("A11y VK functional keys check",
		action,
		uc,
		nil,
	)(ctx); err != nil {
		s.Fatal("Failed to validate A11y virtual keyboard: ", err)
	}
}
