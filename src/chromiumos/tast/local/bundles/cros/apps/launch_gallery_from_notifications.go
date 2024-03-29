// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/bundles/cros/apps/galleryapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchGalleryFromNotifications,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify Gallery launches correctly when opening image from notifications",
		Contacts: []string{
			"backlight-swe@google.com",
		},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"gear_wheels_4000x3000_20200624.jpg", "download_link.html"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.LoggedIn,
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
				ExtraAttr:         []string{"group:mainline"},
			}, {
				Name:    "unstable",
				Fixture: fixture.LoggedIn,
				// b:238260020 - disable aged (>1y) unpromoted informational tests
				// ExtraAttr:         []string{"group:mainline", "informational"},
				ExtraHardwareDeps: hwdep.D(pre.AppsUnstableModels),
			}, {
				Name:              "lacros",
				Fixture:           fixture.LacrosLoggedIn,
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"group:mainline"},
				ExtraHardwareDeps: hwdep.D(pre.AppsStableModels),
			},
		},
	})
}

// LaunchGalleryFromNotifications verifies Gallery opens when Chrome notifications are clicked.
func LaunchGalleryFromNotifications(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn

	const (
		testImageFileName    = "gear_wheels_4000x3000_20200624.jpg"
		uiTimeout            = 20 * time.Second
		downloadCompleteText = "Download complete"
	)
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	testImageFileLocation := filepath.Join(downloadsPath, testImageFileName)
	defer os.Remove(testImageFileLocation)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Create cleanup context to ensure UI tree dumps correctly.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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
