// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/input"

	// "chromiumos/tast/errors"
	// "chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"

	// "chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"

	// "chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragDrop,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify drag drop from files app works",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Timeout: 4 * time.Minute,
		Attr:    []string{"group:mainline", "informational"},
		Data: []string{
			"drag_drop_pwa_manifest.json",
			"drag_drop_pwa_service.js",
			"drag_drop_pwa_icon.png",
			"drag_drop_pwa_window.html",
		},
		SoftwareDeps: []string{"chrome"},
		SearchFlags: []*testing.StringPair{
			{
				Key:   "feature_id",
				Value: "screenplay-4acc1d8c-a491-49ae-acb8-d3e7f29a510a",
			},
		},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func DragDrop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the test API connection: ", err)
	}

	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)

	server := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := server.Shutdown(ctx); err != nil {
			s.Log("Failed to stop http server: ", err)
		}
	}(cleanupCtx)

	bt := s.Param().(browser.Type)
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, bt, "http://localhost:8080/drag_drop_pwa_window.html")
	if err != nil {
		s.Fatal("Failed to load PWA for URL: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	window, err := ash.WaitForAnyWindowWithTitle(ctx, tconn, "awaiting drop.")
	if err != nil {
		s.Fatal("Failed to wait for PWA window to appear: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateRightSnapped); err != nil {
		s.Fatal("Failed to snap Test PWA to the right: ", err)
	}
	// After the window has been snapped to the right the bounds change. However,
	// the bounds stored in `w` are cached, so update the bounds again.
	window, err = ash.GetWindow(ctx, tconn, window.ID)
	if err != nil {
		s.Fatal("Failed to get the updated window bounds: ", err)
	}
	dstPoint := window.TargetBounds.CenterPoint()

	// defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get users MyFiles path: ", err)
	}

	// Setup the test file.
	const textFile = "test.txt"
	testFileLocation := filepath.Join(myFilesPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Close(cleanupCtx)

	window, err = ash.WaitForAnyWindowWithTitle(ctx, tconn, filesapp.FilesTitlePrefix)
	if err != nil {
		s.Fatal("Failed to wait for Files app window to appear: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateLeftSnapped); err != nil {
		s.Fatal("Failed to snap Files app to the left: ", err)
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

	if err := verifyDroppedFileMatchesDraggedFile(ctx, tconn, textFile); err != nil {
		s.Fatal("Failed verifying the dropped file matches the drag file: ", err)
	}
}

// verifyDroppedFileMatchesDraggedFile observes the extensions window title for changes.
// If the title changes to drop registered, the file names are compared to ensure file data is transferred.
func verifyDroppedFileMatchesDraggedFile(ctx context.Context, tconn *chrome.TestConn, createdFileName string) error {
	window, err := ash.FindOnlyWindow(ctx, tconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "drop registered:")
	})
	if err != nil {
		return errors.Wrap(err, "failed registering drop on PWA, title has not changed")
	}
	titleMatcher := regexp.MustCompile("drop registered: (" + strings.ReplaceAll(createdFileName, ".", "\\.") + ")")
	actualDroppedFileName := titleMatcher.FindStringSubmatch(window.Title)
	if len(actualDroppedFileName) != 2 {
		testing.ContextLog(ctx, actualDroppedFileName)
		return errors.Errorf("failed to extract filename from title: %q", window.Title)
	}
	if createdFileName != actualDroppedFileName[1] {
		return errors.Errorf("failed dropped file doesnt match dragged file, got: %q; want: %q", actualDroppedFileName[1], createdFileName)
	}
	return nil
}
