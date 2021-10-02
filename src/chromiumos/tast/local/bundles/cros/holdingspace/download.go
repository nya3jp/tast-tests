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
	})
}

// Download verifies download behavior in holding space. It is expected that
// initiating a download will result in an item being added to holding space
// from which the user can pause/resume the download.
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

	// Cache the desired `filename`, `filesize`, and `filelocation` of the
	// download. Note that the `filesize` should be large enough to provide
	// sufficient time to pause/resume the download from holding space.
	filename := "download.txt"
	filesize := strconv.Itoa(500 * 1024 * 1024) // 500 MB.
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

	// Right click the download chip to show the context menu. Note that the
	// download chip is currently bound to an in-progress download.
	if err := ui.RightClick(api.FindDownloadChip("Downloading " + filename))(ctx); err != nil {
		s.Error("Failed to right click download chip: ", err)
	}

	// Left click the "Pause" context menu item. Note that this will result in the
	// underlying download being paused and the context menu being closed.
	if err := ui.LeftClick(api.FindContextMenuItem("Pause"))(ctx); err != nil {
		s.Error("Failed to left click context menu item: ", err)
	}

	// Right click the download chip to show the context menu. Note that the
	// download chip is currently bound to a paused download.
	if err := ui.RightClick(api.FindDownloadChip("Download paused " + filename))(ctx); err != nil {
		s.Error("Failed to right click download chip: ", err)
	}

	// Left click the "Resume" context menu item. Note that this will result in
	// the underlying download being resumed and the context menu being closed.
	if err := ui.LeftClick(api.FindContextMenuItem("Resume"))(ctx); err != nil {
		s.Error("Failed to left click context menu item: ", err)
	}

	// Wait for the download to complete.
	if err := ui.WaitUntilExists(api.FindDownloadChip(filename))(ctx); err != nil {
		s.Error("Download chip does not exist: ", err)
	}

	// Remove the file at `filelocation` which is backing the download. Note that
	// this will result in the associated holding space item being removed.
	if err := os.Remove(filelocation); err != nil {
		s.Error("Failed to remove download: ", err)
	}

	// Ensure the download chip is removed with its backing file.
	if err := ui.WaitUntilGone(api.FindDownloadChip(filename))(ctx); err != nil {
		s.Error("Download chip exists: ", err)
	}

	// Ensure the tray continues to exist even after the download has been removed
	// and the holding space is empty. It should remain visible, even when empty,
	// until such time as the user pins their first item to holding space.
	if err := ui.WaitUntilExists(api.FindTray())(ctx); err != nil {
		s.Error("Tray does not exist: ", err)
	}
}
