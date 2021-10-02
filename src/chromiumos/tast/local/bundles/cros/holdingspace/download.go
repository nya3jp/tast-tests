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

	"chromiumos/tast/local/bundles/cros/holdingspace/api"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
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
				filesize: 500 * 1024 * 1024, // 500 MB to allow time to cancel.
				testfunc: testCancel,
			},
		}, {
			Name: "pause_and_resume",
			Val: params{
				filesize: 500 * 1024 * 1024, // 500 MB to allow time to pause/resume.
				testfunc: testPauseAndResume,
			},
		}, {
			Name: "pin",
			Val: params{
				filesize: 0,
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
	// Connect to Chrome.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Connect to the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Ensure the tray does not exist prior adding anything to holding space.
	ui := uiauto.New(tconn)
	if err := ui.EnsureGoneFor(api.FindTray(), 5*time.Second)(ctx); err != nil {
		s.Error("Tray exists: ", err)
	}

	// Cache the `filename`, `filesize`, and `filelocation` of the download.
	filename := "download.txt"
	filesize := strconv.Itoa(s.Param().(params).filesize)
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
	if err := ui.LeftClick(api.FindTray())(ctx); err != nil {
		s.Error("Failed to left click tray: ", err)
	}

	// Perform additional parameterized testing.
	s.Param().(params).testfunc(ctx, s, ui, filename)

	// Remove the file at `filelocation` which is backing the download. Note that
	// this will result in any associated holding space items being removed.
	if err := os.Remove(filelocation); err != nil && !os.IsNotExist(err) {
		s.Error("Failed to remove download: ", err)
	}

	// Ensure all holding space chips associated with the underlying download are
	// removed when the backing file is removed.
	if err := ui.WaitUntilGone(api.FindChip(filename))(ctx); err != nil {
		s.Error("Download chip exists: ", err)
	}
}

// testCancel performs testing of cancelling a download.
func testCancel(
	ctx context.Context, s *testing.State, ui *uiauto.Context, filename string) {
	// Right click the download chip to show the context menu. Note that the
	// download chip is currently bound to an in-progress download.
	err := ui.RightClick(api.FindDownloadChip("Downloading " + filename))(ctx)
	if err != nil {
		s.Error("Failed to right click download chip: ", err)
	}

	// Left click the "Cancel" context menu item. Note that this will result in
	// the underlying download being cancelled and the context menu being closed.
	if err := ui.LeftClick(api.FindContextMenuItem("Cancel"))(ctx); err != nil {
		s.Error("Failed to left click context menu item: ", err)
	}

	// Ensure the download chip is removed with its backing file.
	if err := ui.WaitUntilGone(api.FindDownloadChip(filename))(ctx); err != nil {
		s.Error("Download chip exists: ", err)
	}
}

// testPauseAndResume performs testing of pausing and resuming a download.
func testPauseAndResume(
	ctx context.Context, s *testing.State, ui *uiauto.Context, filename string) {
	// Right click the download chip to show the context menu. Note that the
	// download chip is currently bound to an in-progress download.
	err := ui.RightClick(api.FindDownloadChip("Downloading " + filename))(ctx)
	if err != nil {
		s.Error("Failed to right click download chip: ", err)
	}

	// Left click the "Pause" context menu item. Note that this will result in the
	// underlying download being paused and the context menu being closed.
	if err := ui.LeftClick(api.FindContextMenuItem("Pause"))(ctx); err != nil {
		s.Error("Failed to left click context menu item: ", err)
	}

	// Right click the download chip to show the context menu. Note that the
	// download chip is currently bound to a paused download.
	err = ui.RightClick(api.FindDownloadChip("Download paused " + filename))(ctx)
	if err != nil {
		s.Error("Failed to right click download chip: ", err)
	}

	// Left click the "Resume" context menu item. Note that this will result in
	// the underlying download being resumed and the context menu being closed.
	if err := ui.LeftClick(api.FindContextMenuItem("Resume"))(ctx); err != nil {
		s.Error("Failed to left click context menu item: ", err)
	}

	// Wait for the download to complete.
	err = ui.WaitUntilExists(api.FindDownloadChip(filename))(ctx)
	if err != nil {
		s.Error("Download chip does not exist: ", err)
	}
}

// testPin performs testing of pinning a download.
func testPin(
	ctx context.Context, s *testing.State, ui *uiauto.Context, filename string) {
	// Right click the download chip to show the context menu. Note that this will
	// wait until the underlying download has completed.
	if err := ui.RightClick(api.FindDownloadChip(filename))(ctx); err != nil {
		s.Error("Failed to right click download chip: ", err)
	}

	// Left click the "Pin" context menu item. Note that this will result in
	// a pinned holding space item being created for the underlying download and
	// the context menu being closed.
	if err := ui.LeftClick(api.FindContextMenuItem("Pin"))(ctx); err != nil {
		s.Error("Failed to left click context menu item: ", err)
	}

	// Ensure the pinned file chip is created.
	err := ui.WaitUntilExists(api.FindPinnedFileChip(filename))(ctx)
	if err != nil {
		s.Error("Pinned file chip does not exist: ", err)
	}
}
