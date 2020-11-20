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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/mouse"
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
	defer files.Release(ctx)

	// Clicking on a file is not enough as the clicks can be too quick for FileInfo
	// to be added to the drop event, this leads to an empty event. Clicking the
	// file and checking the Action Bar we can guarantee FileInfo exists on the
	// drop event.
	srcPoint, err := tickCheckboxForFile(ctx, files, textFile)
	if err != nil {
		s.Fatal("Failed selecting file: ", err)
	}

	// Get connection to foreground extension to verify changes.
	dropTargetURL := "chrome-extension://" + dropTargetExtID + "/window.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(dropTargetURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", dropTargetURL, err)
	}
	defer conn.Close()

	if err := waitForFileToBeSelected(ctx, conn, files); err != nil {
		s.Fatal("Failed waiting for the drag drop extension to be ready: ", err)
	}

	dstPoint := coords.Point{X: 100, Y: 100}
	if err := mouse.Drag(ctx, tconn, srcPoint, dstPoint, time.Second); err != nil {
		s.Fatal("Failed to send drag events: ", err)
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

// waitForFileToBeSelected makes sure the extension title has loaded JavaScript properly.
// This also waits for the files app listbox to stabilize.
func waitForFileToBeSelected(ctx context.Context, conn *chrome.Conn, f *filesapp.FilesApp) error {
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, "window.document.title == 'awaiting drop.'", 5*time.Second); err != nil {
		return errors.Wrap(err, "failed waiting for javascript to update window.document.title")
	}

	// Get the listbox which has the list of files.
	listBox, err := f.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find listbox")
	}
	defer listBox.Release(ctx)

	// Setup a watcher to wait for the selected files to stabilize.
	ew, err := ui.NewWatcher(ctx, listBox, ui.EventTypeActiveDescendantChanged)
	if err != nil {
		return errors.Wrap(err, "failed getting a watcher for the files listbox")
	}
	defer ew.Release(ctx)

	// Check the listbox for any Activedescendantchanged events occurring in a 2 second interval.
	// If any events are found continue polling until 10s is reached.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, 2*time.Second)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed waiting %v for listbox to stabilize", 10*time.Second)
	}

	return nil
}

// tickCheckboxForFile clicks the checkbox on a file and waits for selected label.
func tickCheckboxForFile(ctx context.Context, f *filesapp.FilesApp, fileName string) (coords.Point, error) {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to create keyboard")
	}
	defer ew.Close()

	// Hold Ctrl during selection.
	if err := ew.AccelPress(ctx, "Ctrl"); err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to press Ctrl")
	}
	defer ew.AccelRelease(ctx, "Ctrl")

	// Wait for the file.
	params := ui.FindParams{
		Name: fileName,
		Role: ui.RoleTypeStaticText,
	}
	file, err := f.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return coords.Point{}, errors.Wrapf(err, "failed finding file %q: %v", fileName, err)
	}
	defer file.Release(ctx)

	if err := file.LeftClick(ctx); err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to left click file")
	}

	params = ui.FindParams{
		Role: ui.RoleTypeStaticText,
		Name: "1 file selected",
	}
	if err := f.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		return coords.Point{}, errors.Wrap(err, "failed to find expected selection label")
	}

	return file.Location.CenterPoint(), nil
}
