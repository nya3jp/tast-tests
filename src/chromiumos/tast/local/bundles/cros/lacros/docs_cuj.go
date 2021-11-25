// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DocsCUJ,
		Desc:         "Runs Google Docs CUJ against both ash-chrome and lacros-chrome",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		Timeout:      6 * time.Minute,
	})
}

const (
	// Google Docs with 20+ pages of random text with 50 comments. The URL points to a comment and will skip
	// down to the comment once the page is fully loaded.
	docsURLToComment = "https://docs.google.com/document/d/1MW7lAk9RZ-6zxpObNwF0r80nu-N1sXo5f7ORG4usrJQ/edit?disco=AAAAP6EbSF8"
)

func DocsCUJ(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(launcher.FixtValue)

	cleanup, err := lacros.SetupPerfTest(ctx, f.TestAPIConn(), "lacros.DocsCUJ")
	if err != nil {
		s.Fatal("Failed to set up lacros perf test: ", err)
	}
	defer cleanup(ctx)

	pv := perf.NewValues()

	// Run against ash-chrome.
	if loadTime, visibleLoadTime, err := runDocsPageLoad(ctx, f.TestAPIConn(), docsURLToComment, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		return lacros.SetupCrosTestWithPage(ctx, f, url)
	}); err != nil {
		s.Error("Failed to run ash-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "docs.load.ash",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, loadTime.Seconds())

		pv.Set(perf.Metric{
			Name:      "docs.load_and_visible.ash",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, visibleLoadTime.Seconds())
	}

	// Run against lacros-chrome from Shelf
	if loadTime, visibleLoadTime, err := runDocsPageLoad(ctx, f.TestAPIConn(), docsURLToComment, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		conn, _, _, cleanup, err := setupLacrosShelfTestWithPage(ctx, f, url)
		return conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run lacros-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "docs.load.lacros_shelf",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, loadTime.Seconds())

		pv.Set(perf.Metric{
			Name:      "docs.load_and_visible.lacros_shelf",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, visibleLoadTime.Seconds())
	}

	// Grab Lacros Shelf log to assist debugging before exiting.
	if errCopy := fsutil.CopyFile(filepath.Join(launcher.LacrosUserDataDir, "lacros.log"), filepath.Join(s.OutDir(), "lacros-shelf.log")); errCopy != nil {
		s.Log("Failed to copy lacros.log from LacrosUserDataDir to the OutDir ", errCopy)
	}

	// Run against lacros-chrome.
	if loadTime, visibleLoadTime, err := runDocsPageLoad(ctx, f.TestAPIConn(), docsURLToComment, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		conn, _, _, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, f, url)
		return conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run lacros-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "docs.load.lacros",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, loadTime.Seconds())

		pv.Set(perf.Metric{
			Name:      "docs.load_and_visible.lacros",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, visibleLoadTime.Seconds())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

// runDocsPageLoad navigates to the Google Docs URL page and benchmark the time to load it.
// It returns the page loading time (loadTime) and the user-visible milestone of loading the page
// (visibleLoadTime), given the latter really captures the real user experience speacially when
// loading large pages.
// tconn is a test connection to the ash-chrome test connection.
func runDocsPageLoad(
	ctx context.Context,
	tconn *chrome.TestConn,
	url string,
	setup func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error)) (time.Duration, time.Duration, error) {
	conn, cleanup, err := setup(ctx, chrome.BlankURL)
	if err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to open a new tab")
	}
	defer cleanup(ctx)

	w, err := lacros.FindFirstBlankWindow(ctx, tconn)
	if err != nil {
		return 0.0, 0.0, err
	}

	// Maximize browser window (either ash-chrome or lacros) to ensure a consistent state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to maximize window")
	}

	start := time.Now()

	// Navigate the blankpage to the document file to be loaded.
	// This blocks until the loading is completed and is a important metric already.
	if err := conn.Navigate(ctx, url); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to navigate a blankpage to the URL")
	}

	// Save load time perf data as well.
	loadTime := time.Since(start)

	// Check whether comment link is loaded and visible.
	// WaitForExpr has to be used since the comment link is not updated immediately.
	const expr = `document.querySelector("#docos-stream-view > div.docos-docoview-tesla-conflict.docos-docoview-resolve-button-visible.docos-anchoreddocoview.docos-docoview-active.docos-docoview-active-experiment")
	.innerText`
	if err := conn.WaitForExpr(ctx, expr); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to wait the comment link to be loaded and visible")
	}

	visibleLoadTime := time.Since(start)

	return time.Duration(loadTime), time.Duration(visibleLoadTime), nil
}

// TODO(tvignatti): move cooldownConfig, CleanupCallback and setupLacrosShelfTestWithPage to perftest.go
// cooldownConfig is the configuration used to wait for the stabilization of CPU
// shared between ash-chrome test setup and lacros-chrome test setup.
var cooldownConfig = cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)

// setupLacrosShelfTestWithPage opens a lacros-chrome page from the Shelf after waiting for a stable environment (CPU temperature, etc).
func setupLacrosShelfTestWithPage(ctx context.Context, f launcher.FixtValue, url string) (
	retConn *chrome.Conn, retTConn *chrome.TestConn, retL *launcher.LacrosChrome, retCleanup lacros.CleanupCallback, retErr error) {
	cr := f.Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to connect to the test API connection")
	}

	l, err := lacros.LaunchFromShelf(ctx, tconn, f.LacrosPath())
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to launch lacros")
	}

	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to find new tab")
	}

	if err := conn.Navigate(ctx, url); err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to navigate to the URL")
	}

	// Move the cursor away from the Shelf to make sure the tooltip won't interfere with the performance.
	if err := mouse.Move(tconn, coords.NewPoint(0, 0), 0)(ctx); err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to move the mouse to the top-left corner of the screen")
	}

	cleanup := func(ctx context.Context) error {
		conn.CloseTarget(ctx)
		conn.Close()
		l.Close(ctx)
		return nil
	}

	if err := cpu.WaitUntilStabilized(ctx, cooldownConfig); err != nil {
		return nil, nil, nil, nil, err
	}

	return conn, tconn, l, cleanup, nil
}
