// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchGalleryFromNotifications,
		Desc: "Verify Gallery launches correctly when opening image from notifications",
		Contacts: []string{
			"backlight-swe@google.com",
			"benreich@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"gear_wheels_4000*3000_20200624.jpg", "download_link.html"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{
			{
				Name:              "clamshell_stable",
				ExtraHardwareDeps: pre.AppsStableModels,
				Val:               false,
			}, {
				Name:              "clamshell_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Val:               false,
			}, {
				Name:              "tablet_stable",
				ExtraHardwareDeps: pre.AppsStableModels,
				Val:               true,
			}, {
				Name:              "tablet_unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
				Val:               true,
			},
		},
	})
}

// LaunchGalleryFromNotifications verifies Gallery opens when Chrome notifications are clicked.
func LaunchGalleryFromNotifications(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	const (
		testImageFileName    = "gear_wheels_4000_3000_20200624.jpg" // Chrome changes asterisk to underscore.
		uiTimeout            = 10 * time.Second
		downloadCompleteText = "Download complete"
	)
	pollOpts := testing.PollOptions{Interval: 2 * time.Second, Timeout: uiTimeout}
	testImageFileLocation := filepath.Join(filesapp.DownloadPath, testImageFileName)
	defer os.Remove(testImageFileLocation)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Create cleanup context to ensure UI tree dumps correctly.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	isTabletEnabled := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletEnabled)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	conn, err := cr.NewConn(ctx, filepath.Join(server.URL, "download_link.html"))
	if err != nil {
		s.Fatal("Failed navigating to image on local server: ", err)
	}
	defer conn.Close()

	_, err = ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(downloadCompleteText))
	if err != nil {
		s.Fatalf("Failed waiting %v for download notification", uiTimeout)
	}

	params := ui.FindParams{
		Name: downloadCompleteText,
		Role: ui.RoleTypeStaticText,
	}
	if err := ui.StableFindAndClick(ctx, tconn, params, &pollOpts); err != nil {
		s.Fatal("Failed finding notification and clicking it: ", err)
	}

	s.Log("Wait for Gallery shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	s.Log("Wait for Gallery app rendering")
	// Use image section to verify Gallery App rendering.
	params = ui.FindParams{
		Role: ui.RoleTypeImage,
		Name: testImageFileName,
	}
	if _, err = ui.FindWithTimeout(ctx, tconn, params, 60*time.Second); err != nil {
		s.Fatal("Failed to render Gallery or open image: ", err)
	}
}
