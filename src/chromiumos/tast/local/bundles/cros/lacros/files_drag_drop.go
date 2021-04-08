// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesDragDrop,
		Desc:         "Tests dragging a file from Files app to a LaCrOS browser",
		Contacts:     []string{"lacros-team@google.com", "chromeos-files-syd@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosStartedByData",
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
		Data:         []string{launcher.DataArtifact},
	})
}

func FilesDragDrop(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	ashTestConn := s.FixtValue().(launcher.FixtData).TestAPIConn

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Setup the test file.
	const textFile = "test.txt"
	testFileLocation := filepath.Join(filesapp.MyFilesPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	l, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData), s.DataPath(launcher.DataArtifact))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(cleanupCtx)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, ashTestConn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Snap the LaCrOS window to the left half of the screen.
	w, err := snapWindowToOrientation(ctx, ashTestConn, "about:blank", ash.WindowStateLeftSnapped)
	if err != nil {
		s.Fatal("Failed snapping the lacros window to the left: ", err)
	}
	dstPoint := coords.Point{X: w.BoundsInRoot.Left + (w.BoundsInRoot.Width / 2), Y: w.BoundsInRoot.Top + (w.BoundsInRoot.Height / 2)}

	// Snap the Files app to the right half of the screen.
	if _, err := snapWindowToOrientation(ctx, ashTestConn, "Files -", ash.WindowStateRightSnapped); err != nil {
		s.Fatal("Failed snapping the lacros window to the left: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// The Files App may show a welcome banner on launch to introduce the user to new features.
	// Increase polling options to give UI more time to stabilize in the event that a banner is shown.
	dragDropAction := files.WithTimeout(5*time.Second).WithInterval(time.Second).DragAndDropFile(textFile, dstPoint, kb)
	if err := files.PerformActionAndRetryMaximizedOnFail(dragDropAction)(ctx); err != nil {
		s.Fatal("Failed to drag and drop: ", err)
	}

	// TODO(benreich): Update with assertion that LaCrOS has received the drop.
}

func snapWindowToOrientation(ctx context.Context, tconn *chrome.TestConn, title string, state ash.WindowStateType) (*ash.Window, error) {
	// Find the window that contains the supplied title.
	w, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		return strings.Contains(window.Title, title)
	})
	if err != nil {
		return nil, err
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, state); err != nil {
		return nil, err
	}

	// Find the window again as the coordinates have changed once the window has snapped.
	w, err = ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		return strings.Contains(window.Title, title)
	})
	if err != nil {
		return nil, err
	}

	return w, nil
}
