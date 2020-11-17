// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CopyImage,
		Desc: "Verify copying images to the system clipboard from files app works",
		Contacts: []string{
			"nohe@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"drag_drop_manifest.json", "drag_drop_background.js", "drag_drop_window.js", "drag_drop_window.html", "files_app_test.png"},
		SoftwareDeps: []string{"chrome"},
	})
}

func CopyImage(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("", "tast.filemanager.DragDropExtension")
	if err != nil {
		s.Fatal("Failed creating temp extension directory: ", err)
	}
	defer os.RemoveAll(extDir)

	copyImageTargetExtID, err := setupCopyImageExtension(ctx, s, extDir)
	if err != nil {
		s.Fatal("Failed to setup the copy image extension: ", err)
	}

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir), chrome.EnableFeatures("EnableFilesAppCopyImage"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup the test image.
	const (
		previewImageFile = "files_app_test.png"
	)

	imageFileLocation := filepath.Join(filesapp.DownloadPath, previewImageFile)
	if err := fsutil.CopyFile(s.DataPath(previewImageFile), imageFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", imageFileLocation, err)
	}
	defer os.Remove(imageFileLocation)

	// Create a folder to paste into.
	testFolderName := "test_folder"
	if testTempFolderName, err := ioutil.TempDir(filesapp.DownloadPath, testFolderName); err != nil {
		s.Fatal("Failed to create folder: ", err)
	} else {
		testFolderName = filepath.Base(testTempFolderName)
		defer os.RemoveAll(testTempFolderName)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer files.Release(ctx)

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Right click on the image and copy to the clipboard.
	if err := files.WaitForFile(ctx, previewImageFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}
	if err := files.SelectContextMenu(ctx, previewImageFile, filesapp.Copy); err != nil {
		s.Fatal("Selecting copy on the file failed: ", err)
	}

	if err := files.OpenFile(ctx, testFolderName); err != nil {
		s.Fatal("Opening test dir failed: ", err)
	}

	// Paste the file to a new folder to ensure copy and paste between directories works.
	if err := files.ClickMoreMenuItem(ctx, []string{filesapp.Paste}); err != nil {
		s.Fatal("Could not paste to new folder: ", err)
	}

	// Verify the paste event actually occurred.
	if err := files.WaitForFile(ctx, previewImageFile, 10*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	// extWindow coordinates comes from the drap_drop_background.js file which instructs the extension to
	// create a window starting at 0,0 and to be 300 pixels tall by 300 pixels wide.
	extWindow := coords.Point{X: 100, Y: 100}
	if err := mouse.Click(ctx, tconn, extWindow, mouse.LeftButton); err != nil {
		s.Fatal("Failed to click inside the extension window: ", err)
	}
	kb.Accel(ctx, "Ctrl+v")

	copyTargetURL := "chrome-extension://" + copyImageTargetExtID + "/window.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(copyTargetURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", copyTargetURL, err)
	}
	defer conn.Close()

	if err := verifyPastedData(ctx, conn, previewImageFile); err != nil {
		s.Fatal("Failed verifying the pasted file matches the copied file: ", err)
	}
}

// setupCopyImageExtension moves the extension files into the extension directory and returns the extension ID.
func setupCopyImageExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
	for _, name := range []string{"manifest.json", "background.js", "window.js", "window.html"} {
		if err := fsutil.CopyFile(s.DataPath("drag_drop_"+name), filepath.Join(extDir, name)); err != nil {
			return "", errors.Wrapf(err, "failed to copy extension %q: %v", name, err)
		}
	}
	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to compute extension ID for %q: %v", extDir, err)
	}

	return extID, nil
}

func verifyPastedData(ctx context.Context, conn *chrome.Conn, filename string) error {
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, "window.document.title.startsWith('paste registered:')", 5*time.Second); err != nil {
		return errors.Wrap(err, "failed registering paste on extension, title has not changed")
	}

	var actuallyPastedFileName string
	if err := conn.Eval(ctx, "window.document.title.replace('paste registered:', '')", &actuallyPastedFileName); err != nil {
		return errors.Wrap(err, "failed retrieving the paste window title")
	}

	if filename != actuallyPastedFileName {
		return errors.Errorf("failed pasted file doesnt match dragged file, got: %q; want: %q", actuallyPastedFileName, filename)
	}

	var fileType string
	if err := conn.Eval(ctx, "window.myData.files[0].type", &fileType); err != nil {
		return errors.Wrap(err, "failed retrieving the custom window property for file type")
	}

	if fileType != "image/png" {
		return errors.Errorf("failed pasted file type doesnt match dragged file, got: %q; want: %q", fileType, "image/png")
	}

	var html string
	if err := conn.Eval(ctx, "window.myData['text/html']", &html); err != nil {
		return errors.Wrap(err, "failed retrieving the custom window property for html")
	}

	if html != expectedHTMLData() {
		return errors.Errorf("failed pasted file html doesnt match dragged file, got: %q; want: %q", html, expectedHTMLData())
	}

	return nil
}

func expectedHTMLData() string {
	return "<img src='data:image/png;base64,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'>"
}
