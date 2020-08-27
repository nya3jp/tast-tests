// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardGuest,
		Desc:         "Checks that both physical and virtual keyboards work in guest mode",
		Contacts:     []string{"essential-inputs-team@google.com", "shengjun@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: pre.InputsStableModels,
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}},
	})
}

// KeyboardGuest checks that both physical keyboard and virtual keyboard work in guest mode.
func KeyboardGuest(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard", "--force-tablet-mode=touch_view"), chrome.GuestLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Use virtual keyboard to type keywords.
	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to virtual keyboard UI failed: ", err)
	}
	defer kconn.Close()

	if err := kconn.Eval(ctx, "chrome.inputMethodPrivate.showInputView()", nil); err != nil {
		s.Fatal("Failed to show virtual keyboard via JS: ", err)
	}

	s.Log("Wait for input decoder running")
	if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
		s.Fatal("Failed to wait for decoder: ", err)
	}

	if err := vkb.TapKeysJS(ctx, kconn, strings.Split("help", "")); err != nil {
		s.Fatal("Failed to input via vk javascript: ", err)
	}

	// Use physical keyboard to press ENTER.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to accel(Enter): ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isTargetAvailable, err := cr.IsTargetAvailable(ctx, func(t *target.Info) bool { return t.Title == "Explore" })
		if err != nil {
			return err
		}
		if !isTargetAvailable {
			return errors.New("failed to find help app target")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to Launch Help: ", err)
	}
}
