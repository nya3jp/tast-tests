// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LanguageModelDownload,
		Desc:         "Validity check on downloading input language models",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
	})
}

func LanguageModelDownload(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Brand new login.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	fs := dutfs.NewClient(tconn)

	// TODO file location
	lmFileLocation := ""

	const loopTimes = 10
	for i := 0; i < loopTimes; i++ {
		// 1. Set current input method to PINYIN_CHINESE_SIMPLIFIED.
		imeCode := ime.IMEPrefix + ime.INPUTMETHOD_PINYIN_CHINESE_SIMPLIFIED
		s.Logf("Set current input method to: %s", imeCode)
		if err := ime.AddAndSetInputMethod(ctx, tconn, imeCode); err != nil {
			s.Fatalf("Failed to set input method to %s: %v: ", imeCode, err)
		}

		// 2. Check downloaded language model file.
		lmFileInfo, err := fs.Stat(ctx, lmFileLocation)
		if err != nil {
			return errors.Wrap(err, "failed to open language model file")
		}

		testing.ContextLog("Language model file size: %d", lmFileInfo.Size())

		// 3. Remove PINYIN_CHINESE_SIMPLIFIED, system will default IME to US-en
		if err := ime.RemoveInputMethod(ctx, tconn, imeCode); err != nil {
			s.Fatalf("Failed to remove input method %s: %v: ", imeCode, err)
		}

		// 4. Should here validate language model file deleted?

		// 5. For each iteration, does it need to re-login?

	}
}
