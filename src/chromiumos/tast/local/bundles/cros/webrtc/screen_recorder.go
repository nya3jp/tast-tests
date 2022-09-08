// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRecorder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that that the screen recorder Tast API works",
		Contacts: []string{
			"alvinjia@google.com",
			"chromeos-engprod-sydney@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      2 * time.Minute,
	})
}

// ScreenRecorder Verifies that that the screen recorder Tast API works.
// Steps:
// 1, Initialize the screen recorder.
// 2, Start the screen recorder.
// 3, Wait for 10 seconds.
// 4, Stop the screen recorder.
// 5, Save the recording file.
// 6, Verify that the recording file exists.
func ScreenRecorder(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	recorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create screen recorder: ", err)
	}

	if err := recorder.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start screen recorder: ", err)
	}
	testing.Sleep(ctx, 10*time.Second)

	if err := recorder.Stop(ctx); err != nil {
		s.Fatal("Failed to stop screen recorder: ", err)
	}

	filename := filepath.Join(s.OutDir(), "recording.webm")
	if err := recorder.SaveInBytes(ctx, filename); err != nil {
		s.Fatal("Failed to save screen recorder: ", err)
	}
	defer os.Remove(filename)

	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		s.Fatal("Failed to file the recording file: ", err)
	}
}
