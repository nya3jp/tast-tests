// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// Struct to contain the virtual keyboard speech test parameters.
type lmTestParams struct {
	imeID          ime.InputMethodCode
	lmFileLocation string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LanguageModelDownload,
		Desc:         "Validity check on downloading input language models",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Params: []testing.Param{
			{
				Name: "chinese_pinyin",
				Val: lmTestParams{
					imeID:          ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
					lmFileLocation: "",
				},
			}, {
				Name: "chinese_pinyin2",
				Val: lmTestParams{
					imeID:          ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED,
					lmFileLocation: "",
				},
			},
		},
	})
}

func LanguageModelDownload(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Brand new login.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=ImeMojoDecoder"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Test parameters that are specific to the current test case.
	lmFileLocation := s.Param().(lmTestParams).lmFileLocation
	imeCode := ime.IMEPrefix + string(s.Param().(lmTestParams).imeID)

	const loopTimes = 10
	for i := 0; i < loopTimes; i++ {
		
		
		// 1. Set current input method to PINYIN_CHINESE_SIMPLIFIED.
		s.Logf("Set current input method to: %s", imeCode)
		if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
		}

		// 2. Check downloaded language model file.
		sf, err := os.Open(lmFileLocation)
		if err != nil {
			s.Fatalf("Failed to find language model file at %s: %v", lmFileLocation, err)
		}
		defer sf.Close()

		fi, err := sf.Stat()
		if err != nil {
			s.Fatalf("Failed to open language model file: ", err)
		}

		testing.ContextLog(ctx, "Language model file size: %d", fi.Size())

		// 3. Remove PINYIN_CHINESE_SIMPLIFIED, system will default IME to US-en
		if err := ime.RemoveInputMethod(ctx, tconn, imeCode); err != nil {
			s.Fatalf("Failed to remove input method %s: %v: ", imeCode, err)
		}
	}
}
