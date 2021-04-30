// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/mafredri/cdp/protocol/input"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/lacros/faillog"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchGalleryFromLacrosDownloads,
		Desc: "Verify Gallery launches correctly when opening image from LaCrOS downloads bar",
		Contacts: []string{
			"backlight-swe@google.com",
			"benreich@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		Fixture:      "lacrosStartedByDataGAIAPool",
		SoftwareDeps: []string{"chrome", "lacros"},
		Data:         []string{"gear_wheels_4000x3000_20200624.jpg", "download_link.html", launcher.DataArtifact},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
			{
				Name:              "unstable",
				ExtraSoftwareDeps: []string{"lacros_unstable"},
			},
		},
	})
}

// LaunchGalleryFromLacrosDownloads verifies Gallery opens when LaCrOS download notifications is clicked.
func LaunchGalleryFromLacrosDownloads(ctx context.Context, s *testing.State) {
	type DOMRect struct {
		Height float64
		Width  float64
		X      float64
		Y      float64
	}

	cr := s.FixtValue().(launcher.FixtData).Chrome

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
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	l, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData), s.DataPath(launcher.DataArtifact))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer func() {
		l.Close(cleanupCtx)
		if err := faillog.Save(s.HasError, l, s.OutDir()); err != nil {
			s.Log("Failed to save lacros logs: ", err)
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Open a new tab and navigate to url.
	autodownloadURL := filepath.Join(server.URL, "download_link.html")
	if _, err := l.Devsess.CreateTarget(ctx, autodownloadURL); err != nil {
		s.Fatalf("Failed to create a new tab with URL %q: %v", autodownloadURL, err)
	}

	downloadsTargetID, err := l.Devsess.CreateTarget(ctx, "chrome://downloads")
	if err != nil {
		s.Fatal("Failed to navigate to chrome://downloads: ", err)
	}

	downloadsConn, err := l.Devsess.NewConn(ctx, downloadsTargetID)
	if err != nil {
		s.Fatal("Failed to attach to chrome://downloads: ", err)
	}

	testing.Sleep(ctx, 10*time.Second)

	// TODO(crbug/1173588): Accessibility nodes are not currently supported, move to accessibility tree query when supported.
	querySelector := "document.querySelector('downloads-manager')?.shadowRoot.querySelector('downloads-item')?.shadowRoot.querySelector('a#file-link')"
	if err := downloadsConn.WaitForExpr(ctx, fmt.Sprintf("%s != null", querySelector), cdputil.ExitOnError, time.Minute); err != nil {
		s.Fatal("Failed to wait for a download link to be available: ", err)
	}

	var boundingClientRect DOMRect
	if _, err := downloadsConn.Eval(ctx, fmt.Sprintf(`new Promise(resolve => {
		const boundingClientRect = %s.getBoundingClientRect();
		resolve({
			height: boundingClientRect.height,
			width: boundingClientRect.width,
			x: boundingClientRect.x,
			y: boundingClientRect.y,
		});
	});`, querySelector), true, &boundingClientRect); err != nil {
		s.Fatal("Failed to get bounding rect for file in chrome://downloads: ", err)
	}

	s.Log(boundingClientRect)

	s.Logf("Dispatching key event {%d, %d}", boundingClientRect.X+(boundingClientRect.Width/2), boundingClientRect.Y+(boundingClientRect.Height/2))
	if err := downloadsConn.DispatchMouseEvent(ctx, &input.DispatchMouseEventArgs{
		Type:   "mousePressed",
		X:      boundingClientRect.X + (boundingClientRect.Width / 2),
		Y:      boundingClientRect.Y + (boundingClientRect.Height / 2),
		Button: input.MouseButtonLeft,
	}); err != nil {
		s.Fatal("Failed to dispatch mouse event to chrome://downloads: ", err)
	}

	testing.Sleep(ctx, 5*time.Second)

	s.Log("Wait for Gallery shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, apps.Gallery.ID, time.Minute); err != nil {
		s.Fatal("Failed to check Gallery in shelf: ", err)
	}

	s.Log("Wait for Gallery app rendering")
	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	imageElementFinder := nodewith.Role(role.Image).Name(testImageFileName)
	// Use image section to verify Gallery App rendering.
	if err := ui.WaitUntilExists(imageElementFinder)(ctx); err != nil {
		s.Fatal("Failed to render Gallery: ", err)
	}
}
