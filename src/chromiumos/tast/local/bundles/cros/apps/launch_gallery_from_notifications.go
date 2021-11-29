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
	"chromiumos/tast/local/bundles/cros/apps/galleryapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchGalleryFromNotifications,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify Gallery launches correctly when opening image from notifications",
		Contacts: []string{
			"backlight-swe@google.com",
		},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"gear_wheels_4000x3000_20200624.jpg", "download_link.html"},
		Fixture:      "chromeLoggedInForEA",
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			}, {
				Name:              "unstable",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			},
		},
	})
}

// LaunchGalleryFromNotifications verifies Gallery opens when Chrome notifications are clicked.
func LaunchGalleryFromNotifications(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	const (
		testImageFileName    = "gear_wheels_4000x3000_20200624.jpg"
		uiTimeout            = 20 * time.Second
		downloadCompleteText = "Download complete"
	)
	testImageFileLocation := filepath.Join(filesapp.DownloadPath, testImageFileName)
	defer os.Remove(testImageFileLocation)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Create cleanup context to ensure UI tree dumps correctly.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	conn, err := cr.NewConn(ctx, filepath.Join(server.URL, "download_link.html"))
	if err != nil {
		s.Fatal("Failed navigating to image on local server: ", err)
	}
	defer conn.Close()

	_, err = ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(downloadCompleteText))
	if err != nil {
		s.Fatalf("Failed waiting %v for download notification", uiTimeout)
	}

	ui := uiauto.New(tconn).WithTimeout(60 * time.Second)
	notificationFinder := nodewith.Role(role.StaticText).Name(downloadCompleteText)

	if err := ui.LeftClick(notificationFinder)(ctx); err != nil {
		s.Fatal("Failed finding notification and clicking it: ", err)
	}

	s.Log("Wait for Gallery shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	s.Log("Wait for Gallery app rendering")
	imageElementFinder := nodewith.Role(role.Image).Name(testImageFileName).Ancestor(galleryapp.RootFinder)
	// Use image section to verify Gallery App rendering.
	if err := ui.WaitUntilExists(imageElementFinder)(ctx); err != nil {
		s.Fatal("Failed to render Gallery: ", err)
	}
}
