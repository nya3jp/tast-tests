// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package inputs contains local Tast tests that exercise Chrome OS essential inputs.
package inputs

import (
	"context"

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
	vkbCtx := vkb.NewContext(cr, tconn)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Check that the keyboard has modifier and tab keys.
	keys := []string{"ctrl", "alt", "caps lock", "tab"}
	if err := uiauto.Combine("trigger A11y virtual keyboard and check functional keys exist",
		vkbCtx.ShowVirtualKeyboard(),
		vkbCtx.WaitForKeysExist(keys),
	)(ctx); err != nil {
		s.Fatal("Failed to validate A11y virtual keyboard: ", err)
	}
}
