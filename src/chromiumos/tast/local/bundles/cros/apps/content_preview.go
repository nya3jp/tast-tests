// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	cpTextFileName  = "contentpreview_20210511.txt"
	cpZipFileName   = "contentpreview_20210511.zip"
	cpVideoFileName = "contentpreview_20210511.webm"
	cpPngFileName   = "contentpreview_20210511.png"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ContentPreview,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test content preview while sharing a single file",
		Contacts: []string{
			"jinrongwu@google.com",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{cpTextFileName, cpZipFileName, cpVideoFileName, cpPngFileName},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:    "lacros",
				Fixture: "lacros",
				Val:     browser.TypeLacros,
			},
			{
				Name:    "chrome",
				Fixture: "chromeLoggedIn",
				Val:     browser.TypeAsh,
			},
		},
	})
}

type subTestData struct {
	name        string
	filePath    string
	thumbnail   string
	shareString string
}

func ContentPreview(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	if s.Param().(browser.Type) == browser.TypeAsh {
		cr = s.FixtValue().(*chrome.Chrome)
	} else {
		cr = s.FixtValue().(lacrosfixt.FixtValue).Chrome()
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Clean up in the end.
	defer func() {
		files, err := ioutil.ReadDir(filesapp.DownloadPath)
		if err != nil {
			s.Log("Failed to read files in Downloads: ", err)
		} else {
			for _, f := range files {
				path := filepath.Join(filesapp.DownloadPath, f.Name())
				if err := os.RemoveAll(path); err != nil {
					s.Logf("Failed to RemoveAll(%q)", path)
				}
			}
		}
	}()

	subTests := []subTestData{
		{
			name:        cpTextFileName,
			filePath:    filepath.Join(filesapp.DownloadPath, cpTextFileName),
			thumbnail:   "", // TODO (melzhang@google.com): to add functions to create thumbnail for the files.
			shareString: cpTextFileName,
		},
		{
			name:        cpZipFileName,
			filePath:    filepath.Join(filesapp.DownloadPath, cpZipFileName),
			thumbnail:   "",
			shareString: cpZipFileName,
		},
		{
			name:        cpVideoFileName,
			filePath:    filepath.Join(filesapp.DownloadPath, cpVideoFileName),
			thumbnail:   "",
			shareString: cpVideoFileName,
		},
		{
			name:        cpPngFileName,
			filePath:    filepath.Join(filesapp.DownloadPath, cpPngFileName),
			thumbnail:   "",
			shareString: cpPngFileName,
		},
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	bubbleView := nodewith.ClassName("SharesheetBubbleView").Role(role.Window)
	shareLabel := nodewith.Name(filesapp.Share).ClassName("Label").Role(role.StaticText).Ancestor(bubbleView)

	for _, data := range subTests {
		for _, button := range []bool{true, false} {
			way := "menu_bar"
			if !button {
				way = "context_menu"
			}
			des := fmt.Sprintf("test_content_preview_from_%s_for_%s", way, data.name)
			s.Run(ctx, des, func(ctx context.Context, s *testing.State) {
				// Copy the file to Downloads for sharing.
				if err := fsutil.CopyFile(s.DataPath(data.name), data.filePath); err != nil {
					s.Fatalf("Failed to copy %s to Downloads, hence skip the test: %s", data.name, err)
				}
				// Open the Files App.
				files, err := filesapp.Launch(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to launch Files app: ", err)
				}

				if err := uiauto.Combine("select file",
					files.OpenDownloads(),
					files.WithTimeout(30*time.Second).WaitForFile(data.name),
					files.SelectFile(data.name))(ctx); err != nil {
					s.Fatal("Failed to select file in Downloads: ", err)
				}

				// Share the test file.
				if button {
					// Click button Share on the menu bar.
					shareButton := nodewith.Name(filesapp.Share).Role(role.Button)
					if err := files.LeftClick(shareButton)(ctx); err != nil {
						s.Fatal("Failed to click button Share: ", err)
					}
				} else {
					// Click context menu item Share.
					if err := files.ClickContextMenuItem(data.name, filesapp.Share)(ctx); err != nil {
						s.Fatal("Failed to click context menu item Share: ", err)
					}
				}

				// This is to exit the share dialog in the end of each sub test.
				defer kb.AccelAction("Esc")(cleanupCtx)
				defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

				fileLabel := nodewith.Name(data.name).ClassName("Label").Role(role.StaticText).Ancestor(bubbleView)
				// Verify the Share dialog and content preview.
				if err := uiauto.Combine("test "+data.name,
					ui.WaitUntilExists(shareLabel),
					ui.WaitUntilExists(fileLabel),
					verifyThumbnail(data))(ctx); err != nil {
					s.Fatalf("Failed to verify content preview for %s: %s", data.name, err)
				}
			})
		}
	}
}

func verifyThumbnail(data subTestData) uiauto.Action {
	return func(ctx context.Context) error {
		// TODO (melzhang@google.com): add code when thumbnail is implemented for content preview.
		return nil
	}
}
