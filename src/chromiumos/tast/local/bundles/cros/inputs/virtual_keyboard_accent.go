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
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/vkb"
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

	its, err := testserver.Launch(ctx, cr)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Failed to create connection to virtual keyboard UI: ", err)
	}
	defer kconn.Close()

	// The input method ID is from:
	// src/chrome/browser/resources/chromeos/input_method/google_xkb_manifest.json
	const (
		inputMethodID = string(ime.INPUTMETHOD_XKB_FR_FRA)
		keyName       = "e"
		accentKeyName = "é"
		languageLabel = "FR"
	)

	inputField := testserver.TextAreaNoCorrectionInputField
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second).WithInterval(1 * time.Second)
	if err := uiauto.Combine("Open VK",
		ime.AddAndSetInputMethodAction(tconn, ime.IMEPrefix+inputMethodID),
		inputField.ClickUntilVKShownAction(tconn),
		uiauto.NamedAction("find language switch key",
			ui.WithTimeout(3*time.Second).WaitUntilExists(nodewith.Name(languageLabel).First())),
	)(ctx); err != nil {
		s.Fatal("Failed to open VK: ", err)
	}

	s.Log("Click and hold key for accent window")
	key, err := vkb.FindKeyNode(ctx, tconn, keyName)
	if err != nil {
		s.Fatalf("Failed to find key %s: %v", keyName, err)
	}

	if err := uiauto.Combine("type accent",
		uiauto.Combine("open accent window",
			mouse.MoveAction(tconn, key.Location.CenterPoint(), 500*time.Millisecond),
			mouse.PressAction(tconn, mouse.LeftButton)),
		// Popup accent window sometimes flash on showing, so using polling instead of DescendantofTimeOut
		uiauto.NamedAction("wait for window to show",
			ui.WithTimeout(10*time.Second).WithInterval(1*time.Second).Poll(func(ctx context.Context) error {
				accentContainer := nodewith.ClassName("goog-container goog-container-vertical accent-container")
				if _, err := ui.Location(ctx, accentContainer); err != nil {
					return errors.Wrap(err, "failed to find the container")
				}

				rect, err := ui.Location(ctx, nodewith.Name(accentKeyName).Role(role.StaticText).Ancestor(accentContainer))
				if err != nil {
					return errors.Wrapf(err, "failed to find accentkey %s", accentKeyName)
				}
				return mouse.Move(ctx, tconn, rect.CenterPoint(), 500*time.Millisecond)
			})),
		mouse.ReleaseAction(tconn, mouse.LeftButton),
		inputField.WaitForValueToBeAction(tconn, accentKeyName),
	)(ctx); err != nil {
		s.Fatal("Failed to type accent: ", err)
	}
}
