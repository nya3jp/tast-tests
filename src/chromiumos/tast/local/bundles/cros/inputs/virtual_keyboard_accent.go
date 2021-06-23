// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream", "informational"},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
		}},
	})
}

func VirtualKeyboardAccent(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardAccent.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	ui := uiauto.New(tconn)

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
	accentContainerFinder := nodewith.ClassName("goog-container goog-container-vertical accent-container")
	accentKeyFinder := nodewith.Ancestor(accentContainerFinder).Name(accentKeyName).Role(role.StaticText)
	languageLabelFinder := vkb.NodeFinder.Name(languageLabel).First()
	keyFinder := vkb.KeyFinder.Name(keyName)

	if err := uiauto.Combine("input accent letter with virtual keyboard",
		its.ClickFieldUntilVKShown(inputField),
		ui.WaitUntilExists(languageLabelFinder),
		ui.MouseMoveTo(keyFinder, 500*time.Millisecond),
		mouse.Press(tconn, mouse.LeftButton),
		// Popup accent window sometimes flash on showing, so using Retry instead of WaitUntilExist.

		ui.WithInterval(time.Second).Retry(10, ui.WaitForLocation(accentContainerFinder)),
		ui.MouseMoveTo(accentKeyFinder, 500*time.Millisecond),
		mouse.Release(tconn, mouse.LeftButton),
		its.WaitForFieldValueToBe(inputField, accentKeyName),
	)(ctx); err != nil {
		s.Fatal("Fail to input accent key on virtual keyboard: ", err)
	}
}
