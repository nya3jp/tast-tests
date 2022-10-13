// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowRecorder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the window recorder Tast API works",
		Contacts: []string{
			"hewer@google.com",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
	})
}

// WindowRecorder tests the window recorder using Screen Share and Screen Capture.
func WindowRecorder(ctx context.Context, s *testing.State) {
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's download path: ", err)
	}

	// chrome.New() should empty the downloads folder but we want to be certain.
	if err := wmp.DeleteAllRecordings(downloadsPath); err != nil {
		s.Fatal("Failed to delete recordings: ", err)
	}

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch the files app: ", err)
	}

	if err := wmp.LaunchScreenCapture(ctx, tconn); err != nil {
		s.Fatal("Failed screen capture test: ", err)
	}
	if err := wmp.RecordWindowScreenCapture(ctx, tconn, downloadsPath); err != nil {
		s.Fatal("Failed to record a window using screen capture: ", err)
	}

	if err := wmp.RecordWindowScreenShare(ctx, tconn, downloadsPath); err != nil {
		s.Fatal("Failed screen share test: ", err)
	}

	if err := wmp.DeleteAllRecordings(downloadsPath); err != nil {
		s.Fatal("Failed to delete recordings: ", err)
	}
}
