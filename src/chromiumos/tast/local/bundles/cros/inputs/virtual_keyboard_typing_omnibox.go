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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
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
			Pre:               pre.VKEnabledTablet(),
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet(),
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "mojo",
			Pre:               pre.IMEServiceEnabled(pre.VKEnabledTablet()),
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

	omniboxInputParams := ui.FindParams{
		Role:       ui.RoleTypeTextField,
		Attributes: map[string]interface{}{"inputType": "url"},
	}
	inputNode, err := ui.FindWithTimeout(ctx, tconn, omniboxInputParams, 10*time.Second)
	if err != nil {
		s.Fatalf("Failed to find Omnibox input with params %v: %v", omniboxInputParams, err)
	}
	defer inputNode.Release(ctx)

	if err := vkb.ClickUntilVKShown(ctx, tconn, inputNode); err != nil {
		s.Fatal("Failed to click the input node and wait for vk shown: ", err)
	}

	if err := vkb.WaitForVKReady(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to wait for virtual keyboard ready: ", err)
	}

	if err := vkb.TapKeys(ctx, tconn, strings.Split(typingKeys, "")); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	// Value change can be a bit delayed after input.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := inputNode.Update(ctx); err != nil {
			return errors.Wrap(err, "failed to update node")
		}

		// When clicking Omnibox, on some devices existing text is highlighted and replaced by new input,
		// while on some other devices, it is not highlighted and inserted new input.
		// So use contains here to avoid flakiness.
		if !strings.Contains(inputNode.Value, typingKeys) {
			return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", inputNode.Value, typingKeys)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard in Omnibox: ", err)
	}

	if err := vkb.HideVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to hide virtual keyboard: ", err)
	}
}
