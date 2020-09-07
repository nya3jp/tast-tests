// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VirtualKeyboardTypingAppsIME,
		Desc:     "Enables manual test using IME decoder of virtual keyboard works in apps; this test only for manual test on G3 VM",
		Contacts: []string{"essential-inputs-team@google.com"},
		// this test is not to be promoted as it is only intended for dev use
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(hwdep.Model("betty")),
		Timeout:      5 * time.Minute,
		Vars: []string{
			"inputs.useIME", // if present will load IME decoder, default is NaCl
		}})
}

func VirtualKeyboardTypingAppsIME(ctx context.Context, s *testing.State) {
	const expectedTypingResult = "awesome"
	typingKeys := strings.Split(expectedTypingResult, "")

	extraArgs := []string{"--enable-virtual-keyboard", "--force-tablet-mode=touch_view"}

	_, ime := s.Var("inputs.useIME")

	if ime {
		extraArgs = append(extraArgs,
			"--enable-features=ImeInputLogicFst,EnableImeSandbox")
		s.Log("Appended IME params")
	}
	cr, err := chrome.New(ctx, chrome.ExtraArgs(extraArgs...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	app := apps.Settings
	s.Logf("Launching %s", app.Name)
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		s.Fatalf("Failed to launch %s: %v", app.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
	}

	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for animation finished: ", err)
	}

	s.Log("Find searchbox input element")
	params := ui.FindParams{
		Role: ui.RoleTypeSearchBox,
		Name: "Search settings",
	}
	searchInputElement, err := ui.FindWithTimeout(ctx, tconn, params, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to find searchbox input field in settings: ", err)
	}
	defer searchInputElement.Release(ctx)

	s.Log("Click searchbox to trigger virtual keyboard")
	if err := vkb.ClickUntilVKShown(ctx, tconn, searchInputElement); err != nil {
		s.Fatal("Failed to click the input node and wait for vk shown: ", err)
	}

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	if !ime {
		s.Log("Wait for decoder running")
		if err := vkb.WaitForDecoderEnabled(ctx, cr, true); err != nil {
			s.Fatal("Failed to wait for virtual keyboard shown up: ", err)
		}
	} else {
		s.Log("No need to wait for decoder running")
	}

	if err := vkb.TapKeys(ctx, tconn, typingKeys); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	// Value change can be a bit delayed after input.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		element, err := ui.FindWithTimeout(ctx, tconn, params, 2*time.Second)
		if err != nil {
			s.Fatal("Failed to find searchbox input field in settings: ", err)
		}
		defer element.Release(ctx)
		inputValueElement, err := element.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeStaticText}, 2*time.Second)
		if err != nil {
			return err
		}
		defer inputValueElement.Release(ctx)
		if inputValueElement.Name != expectedTypingResult {
			return errors.Errorf("failed to input with virtual keyboard (got: %s; want: %s)", inputValueElement.Name, expectedTypingResult)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to input with virtual keyboard: ", err)
	}
}
