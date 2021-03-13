// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAccent,
		Desc:         "Checks that long pressing keys pop up accent window",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
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
		}},
	})
}

func VirtualKeyboardAccent(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	// The input method ID is from:
	// src/chrome/browser/resources/chromeos/input_method/google_xkb_manifest.json
	const (
		inputMethodID = string(ime.INPUTMETHOD_XKB_FR_FRA)
		keyName       = "e"
		accentKeyName = "é"
		languageLabel = "FR"
	)

	if err := ime.AddAndSetInputMethod(ctx, tconn, ime.IMEPrefix+inputMethodID); err != nil {
		s.Fatal("Failed to set input method: ", err)
	}

	inputField := testserver.TextAreaNoCorrectionInputField

	if err := its.ClickFieldUntilVKShown(inputField)(ctx); err != nil {
		s.Fatal("Failed to click input field to show virtual keyboard: ", err)
	}

	params := ui.FindParams{
		Name: languageLabel,
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 3*time.Second); err != nil {
		s.Fatalf("Failed to switch to language %s: %v", inputMethodID, err)
	}

	s.Log("Click and hold key for accent window")
	vk, err := vkb.VirtualKeyboard(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find virtual keyboad automation node: ", err)
	}
	defer vk.Release(ctx)

	keyParams := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: keyName,
	}

	key, err := vk.Descendant(ctx, keyParams)
	if err != nil {
		s.Fatalf("Failed to find key with %v: %v", keyParams, err)
	}
	defer key.Release(ctx)

	if err := mouse.Move(ctx, tconn, key.Location.CenterPoint(), 500*time.Millisecond); err != nil {
		s.Fatalf("Failed to move mouse to key %s: %v", keyName, err)
	}

	if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to press key: ", err)
	}

	// Popup accent window sometimes flash on showing, so using polling instead of DescendantofTimeOut
	s.Log("Waiting for accent window pop up")
	var location coords.Point
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		accentContainer, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "goog-container goog-container-vertical accent-container"}, 1*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the container")
		}
		defer accentContainer.Release(ctx)

		// Wait for pop up window fully positioned
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			containerLocation := accentContainer.Location
			testing.Sleep(ctx, time.Second)
			accentContainer.Update(ctx)
			if accentContainer.Location != containerLocation {
				return errors.New("popup window is not positioned")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return err
		}

		accentKeyParams := ui.FindParams{Name: accentKeyName}
		accentKey, err := accentContainer.Descendant(ctx, accentKeyParams)
		if err != nil {
			return errors.Wrapf(err, "failed to find accentkey with %v", accentKeyParams)
		}
		defer accentKey.Release(ctx)

		if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for animation finished")
		}
		accentKey.Update(ctx)
		location = accentKey.Location.CenterPoint()
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		s.Fatal("Failed to wait for accent window: ", err)
	}

	if err := mouse.Move(ctx, tconn, location, 500*time.Millisecond); err != nil {
		s.Fatalf("Failed to move mouse to key %s: %v", accentKeyName, err)
	}

	if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
		s.Fatal("Failed to release mouse click: ", err)
	}

	if err := its.WaitForFieldValueToBe(inputField, accentKeyName)(ctx); err != nil {
		s.Fatal("Failed to verify input: ", err)
	}
}
