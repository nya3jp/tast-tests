// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/imesettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputMethodManagement,
		Desc:         "Verifies that user can manage input methods in OS settings",
		Contacts:     []string{"shengjun@chromium.org", "myy@google.com", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			ExtraHardwareDeps: hwdep.D(pre.InputsStableModels),
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			ExtraHardwareDeps: hwdep.D(pre.InputsUnstableModels),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func InputMethodManagement(ctx context.Context, s *testing.State) {
	const (
		searchKeyword   = "japanese"                           // Keyword used to search input method.
		inputMethodName = "Japanese with US keyboard"          // Input method should be displayed after search.
		inputMethodCode = string(ime.INPUTMETHOD_NACL_MOZC_US) // Input method code of the input method.
	)

	// This test changes input method, it affects other tests if not cleaned up.
	// Using new Chrome instance to isolate it from other tests.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "InputMethodManagement.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	settings, err := imesettings.LaunchAtInputsSettingsPage(ctx, tconn, cr)
	if err != nil {
		s.Fatal("Failed to launch OS settings and land at inputs setting page: ", err)
	}
	if err := uiauto.Combine("test input method management",
		settings.ClickAddInputMethodButton(),
		settings.SearchInputMethod(keyboard, searchKeyword, inputMethodName),
		settings.SelectInputMethod(inputMethodName),
		settings.ClickAddButtonToConfirm(),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodInstalled(ctx, tconn, inputMethodCode, 60*time.Second)
		},
		settings.RemoveInputMethod(inputMethodName),
		func(ctx context.Context) error {
			return ime.WaitForInputMethodRemoved(ctx, tconn, inputMethodCode, 60*time.Second)
		},
	)(ctx); err != nil {
		s.Fatal("Failed to test input method management: ", err)
	}
}
