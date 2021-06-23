// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/inputs/data"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingIME,
		Desc:         "Checks that virtual keyboard works in different input methods",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:input-tools-upstream", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Pre:          pre.VKEnabledTablet,
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		Timeout:      time.Duration(len(data.VKInputMap)) * time.Minute,
	})
}

func VirtualKeyboardTypingIME(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Log("Failed to create ScreenRecorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "VirtualKeyboardTypingIme.webm"))

	if screenRecorder != nil {
		screenRecorder.Start(ctx, tconn)
	}

	for imeCode, testData := range data.VKInputMap {
		if testData.SkipTest {
			testing.ContextLog(ctx, "Skip testing in input method: ", string(imeCode))
			continue
		}
		s.Run(ctx, string(imeCode), func(ctx context.Context, s *testing.State) {
			defer func() {
				outDir := filepath.Join(s.OutDir(), string(imeCode))
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_tree_"+string(imeCode))
			}()
			// 1 minute should be enough for the retries to avoid flakiness.
			ui := uiauto.New(tconn).WithTimeout(time.Minute)
			if err := ui.Retry(5, its.ValidateVKInputOnField(testserver.TextAreaNoCorrectionInputField, imeCode))(ctx); err != nil {
				s.Fatalf("Failed to validate virtual keyboard input in %s: %v", string(imeCode), err)
			}
		})
	}
}
