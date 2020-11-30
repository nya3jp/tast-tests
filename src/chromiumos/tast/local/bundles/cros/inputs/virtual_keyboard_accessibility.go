// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package inputs contains local Tast tests that exercise Chrome OS essential inputs.
package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAccessibility,
		Desc:         "Checks that the accessibility keyboard displays correctly",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledClamshell,
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledClamshell,
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"group:mainline", "informational"},
		}, {
			Name:              "exp",
			Pre:               pre.VKEnabledClamshellExp,
			ExtraSoftwareDeps: []string{"gboard_decoder"},
			ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
		}},
	})
}

func VirtualKeyboardAccessibility(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	shown, err := vkb.IsShown(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if the virtual keyboard is initially hidden: ", err)
	}
	if shown {
		s.Fatal("Virtual keyboard is shown, but expected it to be hidden")
	}

	if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	if err := vkb.WaitForVKReady(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to wait for virtual keyboard ready: ", err)
	}

	// Check that the keyboard has modifier and tab keys.
	keys := []string{"ctrl", "alt", "caps lock", "tab"}
	if err := vkb.WaitForKeysExist(ctx, tconn, keys); err != nil {
		s.Fatal("Failed to wait for keys to be shown: ", err)
	}
}
