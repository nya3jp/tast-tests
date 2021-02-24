// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingOmnibox,
		Desc:         "Checks that the virtual keyboard works in Chrome browser omnibox",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "exp",
			Pre:               pre.VKEnabledTabletExp,
			ExtraSoftwareDeps: []string{"gboard_decoder"},
			ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
		}}})
}

func VirtualKeyboardTypingOmnibox(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	const typingKeys = "go"
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to connect to test page: ", err)
	}
	defer conn.Close()

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	omniboxInput := nodewith.Role(role.TextField).Attribute("inputType", "url")
	if err := uiauto.Combine("type in the omnibox",
		vkb.ClickUntilVKShownAction(tconn, omniboxInput),
		vkb.WaitForVKReadyAction(tconn, cr),
		vkb.TapKeysAction(tconn, strings.Split(typingKeys, "")),
		// Value change can be a bit delayed after input.
		ui.Poll(func(ctx context.Context) error {
			info, err := ui.Info(ctx, omniboxInput)
			if err != nil {
				return errors.Wrap(err, "failed to update node")
			}

			// When clicking Omnibox, on some devices existing text is highlighted and replaced by new input,
			// while on some other devices, it is not highlighted and inserted new input.
			// So use contains here to avoid flakiness.
			if !strings.Contains(info.Value, typingKeys) {
				return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", info.Value, typingKeys)
			}
			return nil
		}),
		vkb.HideVirtualKeyboardAction(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to type in the omnibox: ", err)
	}
}
