// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	pngFile  = "flatbed_png_color_letter_300_dpi.png"
	jpgFile  = "scan_source.jpg"
	svgFile  = "handwriting_ja_hello.svg"
	wavFile  = "voice_ko_hello.wav"
	oggFile  = "media_session_60sec_test.ogg"
	mp4File  = "720_av1.mp4"
	m4aFile  = "audio.m4a"
	webmFile = "720_vp8.webm"
	pdfFile  = "font-test.pdf"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Quickview,
		Desc:         "Test the functionality of Quick View with different files",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{pngFile, jpgFile, svgFile, wavFile, oggFile, mp4File, m4aFile, webmFile, pdfFile},
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

type quickviewTestDeatil struct {
	name                string
	expectedDescription string
}

// Quickview opens a preview of the file and checks if the Quick View is shown properly
// by verifying a certain description of the file.
func Quickview(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	fa, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files: ", err)
	}
	defer fa.Close(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	testFiles := []quickviewTestDeatil{
		{name: pngFile, expectedDescription: "2550 x 3300"},
		{name: jpgFile, expectedDescription: "2550 x 3507"},
		{name: svgFile, expectedDescription: "3.7 KB"},
		{name: wavFile, expectedDescription: "audio/wav"},
		{name: oggFile, expectedDescription: "audio/ogg"},
		{name: mp4File, expectedDescription: "video/mp4"},
		{name: m4aFile, expectedDescription: "video/mp4"},
		{name: webmFile, expectedDescription: "video/webm"},
		{name: pdfFile, expectedDescription: "application/pdf"},
	}

	ui := uiauto.New(tconn)
	const workingDir = filesapp.DownloadPath

	for _, file := range testFiles {
		var (
			dataSource = s.DataPath(file.name)
			filepath   = filepath.Join(workingDir, file.name)
		)

		if err := fsutil.CopyFile(dataSource, filepath); err != nil {
			s.Fatalf("Failed to copy file %q: %v", file.name, err)
		}

		if err := verify(ui, fa, kb, file.name, file.expectedDescription)(ctx); err != nil {
			s.Fatalf("Failed to verify expected description of %q in quickview: %v", file.name, err)
		}

		if err := os.Remove(filepath); err != nil {
			s.Fatalf("Failed to remove file %q: %v", file.name, err)
		}
	}
}

func verify(ui *uiauto.Context, fa *filesapp.FilesApp, kb *input.KeyboardEventWriter, filename, expectedDescription string) uiauto.Action {
	var (
		back        = nodewith.Name("Back").Role(role.Button)
		description = nodewith.Name(expectedDescription).Role(role.StaticText)
		qvWindow    = nodewith.Name("General info").Ancestor(nodewith.Role(role.Dialog))
	)

	return uiauto.Combine(fmt.Sprintf("quick view and verify description on file: %q", filename),
		fa.OpenDownloads(),
		fa.WaitForFile(filename),
		fa.OpenQuickView(filename),
		fa.WaitUntilExists(description),
		fa.LeftClick(back), // Click back to close quick view window.
		fa.WaitUntilGone(qvWindow),
	)
}
