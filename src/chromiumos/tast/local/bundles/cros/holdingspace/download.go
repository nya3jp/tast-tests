// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/testing"
)

type params struct {
	filesize int
	testfunc func(context.Context, *testing.State, *uiauto.Context, string)
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Download,
		Desc: "Verifies download behavior in holding space",
		Contacts: []string{
			"dmblack@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"download.html"},
		Params: []testing.Param{{
			Name: "cancel",
			Val: params{
				filesize: 500 * 1024 * 1024, // 500 MB to give time to cancel.
				testfunc: testCancel,
			},
		}, {
			Name: "pause_and_resume",
			Val: params{
				filesize: 500 * 1024 * 1024, // 500 MB to give time to pause/resume.
				testfunc: testPauseAndResume,
			},
		}, {
			Name: "pin",
			Val: params{
				filesize: 1, // 1 B.
				testfunc: testPin,
			},
		}},
	})
}

// Download verifies download behavior in holding space. It is expected that
// initiating a download will result in an item being added to holding space
// from which the user can cancel/pause/resume the download. Upon download
// completion, the user should be able to pin the download.
func Download(ctx context.Context, s *testing.State) {
	params := s.Param().(params)

	// Connect to a fresh Chrome instance to ensure holding space first-run state.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Ensure the tray does not exist prior adding anything to holding space.
	ui := uiauto.New(tconn)
	err = ui.EnsureGoneFor(holdingspace.FindTray(), 5*time.Second)(ctx)
	if err != nil {
		s.Error("Tray exists: ", err)
	}

	// Cache the `filename`, `filesize`, and `filelocation` of the download.
	filename := "download.txt"
	filesize := strconv.Itoa(params.filesize)
	filelocation := filepath.Join(filesapp.DownloadPath, filename)
	defer os.Remove(filelocation)

	// Create a local server.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Connect to the local server and navigate to `url`. Note that this will
	// cause the download to be started automatically.
	url := "download.html?filename=" + filename + "&filesize=" + filesize
	conn, err := cr.NewConn(ctx, filepath.Join(server.URL, url))
	if err != nil {
		s.Fatal("Failed to connect to local server: ", err)
	}
	defer conn.Close()

	// Left click the tray to open the bubble.
	if err := ui.LeftClick(holdingspace.FindTray())(ctx); err != nil {
		s.Error("Failed to left click tray: ", err)
	}

	// Perform additional parameterized testing.
	params.testfunc(ctx, s, ui, filename)

	// Remove the file at `filelocation` which is backing the download. Note that
	// this will result in any associated holding space items being removed.
	if err := os.Remove(filelocation); err != nil && !os.IsNotExist(err) {
		s.Error("Failed to remove download: ", err)
	}

	// Ensure all holding space chips associated with the underlying download are
	// removed when the backing file is removed.
	if err := ui.WaitUntilGone(holdingspace.FindChip(filename))(ctx); err != nil {
		s.Error("Chip exists: ", err)
	}
}

// testCancel performs testing of cancelling a download.
func testCancel(
	ctx context.Context, s *testing.State, ui *uiauto.Context, filename string) {
	if err := uiauto.Combine("Test cancel",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		ui.RightClick(holdingspace.FindDownloadChip("Downloading "+filename)),

		// Left click the "Cancel" context menu item. Note that this will result in
		// the underlying download being cancelled and the context menu being
		// closed.
		ui.LeftClick(holdingspace.FindContextMenuItem("Cancel")),

		// Ensure the download chip is removed with its backing file.
		ui.WaitUntilGone(holdingspace.FindDownloadChip(filename)))(ctx); err != nil {
		s.Error("Failed to test cancel: ", err)
	}
}

// testPauseAndResume performs testing of pausing and resuming a download.
func testPauseAndResume(
	ctx context.Context, s *testing.State, ui *uiauto.Context, filename string) {
	if err := uiauto.Combine("Test pause and resume",
		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to an in-progress download.
		ui.RightClick(holdingspace.FindDownloadChip("Downloading "+filename)),

		// Left click the "Pause" context menu item. Note that this will result in
		// the underlying download being paused and the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem("Pause")),

		// Right click the download chip to show the context menu. Note that the
		// download chip is currently bound to a paused download.
		ui.RightClick(holdingspace.FindDownloadChip("Download paused "+filename)),

		// Left click the "Resume" context menu item. Note that this will result in
		// the underlying download being resumed and the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem("Resume")),

		// Wait for the download to complete.
		ui.WaitUntilExists(holdingspace.FindDownloadChip(filename)))(ctx); err != nil {
		s.Error("Failed to test pause and resume: ", err)
	}
}

// testPin performs testing of pinning a download.
func testPin(
	ctx context.Context, s *testing.State, ui *uiauto.Context, filename string) {
	if err := uiauto.Combine("Testing pin",
		// Right click the download chip to show the context menu. Note that this
		// will wait until the underlying download has completed.
		ui.RightClick(holdingspace.FindDownloadChip(filename)),

		// Left click the "Pin" context menu item. Note that this will result in
		// a pinned holding space item being created for the underlying download and
		// the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem("Pin")),

		// Ensure the pinned file chip is created.
		ui.WaitUntilExists(holdingspace.FindPinnedFileChip(filename)))(ctx); err != nil {
		s.Error("Failed to test pin: ", err)
	}
}
