// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	fa "chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AttachFile,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user can attach a file",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedback",
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{fa.PngFile, fa.PdfFile},
		SoftwareDeps: []string{"chrome"},
	})
}

// AttachFile verifies user can attach a file.
func AttachFile(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	// Clean up in the end.
	defer func() {
		files, err := ioutil.ReadDir(downloadsPath)
		if err != nil {
			s.Log("Failed to read files in Downloads: ", err)
		} else {
			for _, f := range files {
				path := filepath.Join(downloadsPath, f.Name())
				if err := os.RemoveAll(path); err != nil {
					s.Logf("Failed to RemoveAll(%q)", path)
				}
			}
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Copy the file to Downloads for uploading purpose.
	files := []string{fa.PngFile, fa.PdfFile}
	for _, fileName := range files {
		if err := fsutil.CopyFile(
			s.DataPath(fileName), filepath.Join(downloadsPath, fileName)); err != nil {
			s.Fatal("Failed to copy file to Downloads: ", err)
		}
	}

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := fa.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and go to share data page: ", err)
	}

	// Find add file button and click.
	addFileButton := nodewith.NameContaining("Add file").Role(
		role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(addFileButton)(ctx); err != nil {
		s.Fatal("Failed to click add file button: ", err)
	}

	// Open Downloads dir and select the png file to upload.
	if err := uiauto.Combine("Open Downloads dir and select PNG file",
		ui.LeftClick(nodewith.Name("Downloads").Role(role.TreeItem)),
		ui.LeftClick(nodewith.NameContaining(fa.PngFile).Role(role.StaticText).First()),
		ui.LeftClick(nodewith.Name("Open").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to open Downloads dir and select PNG file: ", err)
	}

	// Verify the uploaded png file exists.
	pngFileFinder := nodewith.NameContaining(fa.PngFile).Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(pngFileFinder)(ctx); err != nil {
		s.Fatal("Failed to find png file: ", err)
	}

	// Find replace button and click.
	replaceButton := nodewith.NameContaining("Replace").Role(
		role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(replaceButton)(ctx); err != nil {
		s.Fatal("Failed to click replace button: ", err)
	}

	// Upload pdf file.
	if err := uiauto.Combine("Open Downloads dir and select pdf file",
		ui.LeftClick(nodewith.Name("Downloads").Role(role.TreeItem)),
		ui.LeftClick(nodewith.NameContaining(fa.PdfFile).Role(role.StaticText).First()),
		ui.LeftClick(nodewith.Name("Open").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to open Downloads dir and select pdf file: ", err)
	}

	// Verify new uploaded pdf file exists.
	newFile := nodewith.NameContaining(fa.PdfFile).Role(role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(newFile)(ctx); err != nil {
		s.Fatal("Failed to find new file: ", err)
	}
}
