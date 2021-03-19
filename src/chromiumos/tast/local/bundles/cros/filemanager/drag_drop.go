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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DragDrop,
		Desc: "Verify drag drop from files app works",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Timeout:      4 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"drag_drop_manifest.json", "drag_drop_background.js", "drag_drop_window.js", "drag_drop_window.html"},
		SoftwareDeps: []string{"chrome"},
	})
}

func DragDrop(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("", "tast.filemanager.DragDropExtension")
	if err != nil {
		s.Fatal("Failed creating temp extension directory: ", err)
	}
	defer os.RemoveAll(extDir)

	dropTargetExtID, err := setupDragDropExtension(ctx, s, extDir)
	if err != nil {
		s.Fatal("Failed setup of drag drop extension: ", err)
	}

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir))
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

	// Setup the test file.
	const textFile = "test.txt"
	testFileLocation := filepath.Join(filesapp.MyFilesPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Get connection to foreground extension to verify changes.
	dropTargetURL := "chrome-extension://" + dropTargetExtID + "/window.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(dropTargetURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", dropTargetURL, err)
	}
	defer conn.Close()

	// Make sure the extension title has loaded JavaScript properly.
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, "window.document.title == 'awaiting drop.'", 5*time.Second); err != nil {
		s.Fatal("Failed waiting for javascript to update window.document.title: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// The drag drop chrome app defaults to (0,0) with width 300 and height 300 and always on top.
	dstPoint := coords.Point{X: 100, Y: 100}

	// The Files App may show a welcome banner on launch to introduce the user to new features.
	// Increase polling options to give UI more time to stabilize in the event that a banner is shown.
	dragDropAction := files.WithTimeout(5*time.Second).WithInterval(time.Second).DragAndDropFile(textFile, dstPoint, kb)
	if err := files.PerformActionAndRetryMaximizedOnFail(dragDropAction)(ctx); err != nil {
		s.Fatal("Failed to drag and drop: ", err)
	}

	if err := verifyDroppedFileMatchesDraggedFile(ctx, conn, textFile); err != nil {
		s.Fatal("Failed verifying the dropped file matches the drag file: ", err)
	}
}

// setupDragDropExtension moves the extension files into the extension directory and returns the extension ID.
func setupDragDropExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
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

// verifyDroppedFileMatchesDraggedFile observes the extensions window title for changes.
// If the title changes to drop registered, the file names are compared to ensure file data is transferred.
func verifyDroppedFileMatchesDraggedFile(ctx context.Context, conn *chrome.Conn, createdFileName string) error {
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, "window.document.title.startsWith('drop registered:')", 5*time.Second); err != nil {
		return errors.Wrap(err, "failed registering drop on extension, title has not changed")
	}

	var actualDroppedFileName string
	if err := conn.Eval(ctx, "window.document.title.replace('drop registered:', '')", &actualDroppedFileName); err != nil {
		return errors.Wrap(err, "failed retrieving the drag drop window title")
	}

	if createdFileName != actualDroppedFileName {
		return errors.Errorf("failed dropped file doesnt match dragged file, got: %q; want: %q", actualDroppedFileName, createdFileName)
	}

	return nil
}
