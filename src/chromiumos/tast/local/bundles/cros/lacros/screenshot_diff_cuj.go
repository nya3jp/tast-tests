// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenshotDiffCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Screenshot diff tests covering basic CUJs, such as visiting pages and manipulating the browser",
		Contacts:     []string{"lacros-team@google.com", "chromeos-sw-engprod@google.com", "jshargo@chromium.org"},
		Attr:         []string{"group:informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Data:         []string{"color.html"},
		Vars:         []string{"screendiff.debug", "screendiff.dryrun", "goldctl.GoldServiceAccountKey"},
		Params: []testing.Param{
			{
				Name:    "maximize_composited",
				Fixture: "lacrosForceComposition",
			},
			{
				Name:    "maximize_delegated",
				Fixture: "lacrosForceDelegated",
			},
		},
	})
}

func ScreenshotDiffCUJ(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(lacrosfixt.FixtValue)
	tconn, err := f.Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	// Clean up user data dir to ensure a clean start.
	os.RemoveAll(lacros.UserDataDir)

	l, err := lacros.Launch(ctx, f)
	if err != nil {
		s.Fatal("Failed to open a lacros window: ", err)
	}
	defer func() {
		if l != nil {
			l.Close(ctx)
		}
	}()

	s.Log("Checking that Lacros window is visible")
	if err := lacros.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
		// Grab Lacros logs to assist debugging before exiting.
		if errCopy := fsutil.CopyFile(filepath.Join(lacros.UserDataDir, "lacros.log"), filepath.Join(s.OutDir(), "lacros.log")); errCopy != nil {
			s.Log("Failed to copy /home/chronos/user/lacros/lacros.log to the OutDir ", errCopy)
		}

		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	s.Log("Opening a new tab")
	tab, err := l.NewConn(ctx, srv.URL+"/color.html?color=red")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer tab.Close()
	defer tab.CloseTarget(ctx)
	if err := lacros.WaitForLacrosWindow(ctx, tconn, "red"); err != nil {
		s.Fatal("Failed waiting for Lacros to navigate to red color page: ", err)
	}

	config := screenshot.Config{
		DefaultOptions: screenshot.Options{
			SkipWindowMove:      true,
			SkipWindowResize:    true,
			PixelDeltaThreshold: 10,
		},
	}

	differ, err := screenshot.NewDifferFromChrome(ctx, s, f.Chrome(), config)
	if err != nil {
		s.Fatal("Unable to create a screenshot differ: ", err)
	}
	defer differ.DieOnFailedDiffs()

	if err := differ.DiffWindow(ctx, "new window")(ctx); err != nil {
		s.Fatal("Unable to take new window screenshot: ", err)
	}

	tab.Navigate(ctx, srv.URL+"/color.html?color=blue")
	if err := lacros.WaitForLacrosWindow(ctx, tconn, "blue"); err != nil {
		s.Fatal("Failed waiting for Lacros to navigate to blue color page: ", err)
	}

	w, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Unable to get the lacros window: ", err)
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Unable to maximize the lacros window: ", err)
	}

	if err := differ.DiffWindow(ctx, "maximized window")(ctx); err != nil {
		s.Fatal("Unable to take maximized window screenshot: ", err)
	}

	s.Log("Closing lacros-chrome browser")
	if err := l.Close(ctx); err != nil {
		s.Fatal("Failed to close lacros-chrome: ", err)
	}
	l = nil

	if err := ash.WaitForAppClosed(ctx, tconn, apps.Lacros.ID); err != nil {
		s.Fatalf("%s did not close successfully: %s", apps.Lacros.Name, err)
	}
}
