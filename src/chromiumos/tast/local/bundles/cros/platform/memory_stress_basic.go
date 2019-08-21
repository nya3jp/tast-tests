// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	allocPageFilename  = "memory_stress.html"
	javascriptFilename = "memory_stress.js"
	compressRaio       = 0.67
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryStressBasic,
		Desc:     "Create heavy memory pressure and check if oom-killer is invoked",
		Contacts: []string{"vovoy@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		Timeout:  90 * time.Minute,
		Data: []string{
			allocPageFilename,
			javascriptFilename,
		},
		SoftwareDeps: []string{"chrome"},
	})

}

func MemoryStressBasic(ctx context.Context, s *testing.State) {
	const (
		mbPerTab    = 800
		switchCount = 300
	)

	perfValues := perf.NewValues()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	createTabCount, err := openTabCount(s, mbPerTab)
	if err != nil {
		s.Fatal("Failed to get open tab count: ", err)
	}
	testing.ContextLog(ctx, "Tab count to create: ", createTabCount)

	url := server.URL + "/" + allocPageFilename

	if err := openTabs(ctx, s, cr, createTabCount, mbPerTab, url); err != nil {
		s.Fatal("Failed to open tabs: ", err)
	}
	reloadCount, err := switchTabs(ctx, s, cr, switchCount)
	if err != nil {
		s.Fatal("Failed to switch tabs: ", err)
	}

	reloadTabMetric := perf.Metric{
		Name:      "tast_reload_tab_count",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(reloadTabMetric, float64(reloadCount))
	testing.ContextLog(ctx, "Reload tab count:", reloadCount)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

// allocationURL composes the memory allocation URL, paramaters are passed via URL.
func allocationURL(baseURL string, sizeMb int64, randomRatio float64, id int64) string {
	url := baseURL
	url += "?alloc=" + strconv.FormatInt(sizeMb, 10)
	url += "&ratio=" + strconv.FormatFloat(randomRatio, 'f', 3, 64)
	url += "&id=" + strconv.FormatInt(id, 10)
	return url
}

// waitAllocation waits for completion of JavaScript memory allocation.
func waitAllocation(ctx context.Context, conn *chrome.Conn) error {
	const timeout = 60 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Waits for completion of JavaScript allocation.
	expr := "!window.location.href.includes('memory_stress') || document.hasOwnProperty('out') == true"
	if err := conn.WaitForExprFailOnErr(waitCtx, expr); err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			testing.ContextLogf(ctx, "Ignoring tab quiesce timeout (%v)", timeout)
			return nil
		}
		return errors.Wrap(err, "unexpected error waiting for tab quiesce")
	}
	return nil
}

// waitAllocationForURL waits for completion of JavaScript memory allocation on the tab with specified URL.
func waitAllocationForURL(ctx context.Context, cr *chrome.Chrome, url string) error {
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return errors.Wrap(err, "NewConnForTarget failed")
	}
	defer conn.Close()
	if err := waitAllocation(ctx, conn); err != nil {
		return errors.Wrap(err, "waitAllocation failed")
	}
	return nil
}

// openAllocationPage opens a page to allocate many JavaScript objects.
func openAllocationPage(ctx context.Context, url string, cr *chrome.Chrome) error {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "cannot create new renderer")
	}
	defer conn.Close()

	if err := waitAllocation(ctx, conn); err != nil {
		return err
	}
	return nil
}

// openTabCount returns the count of tabs to open.
func openTabCount(s *testing.State, mbPerTab int) (int, error) {
	memPerTab := kernelmeter.NewMemSizeMiB(mbPerTab)
	memInfo, err := kernelmeter.MemInfo()
	if err != nil {
		return 0, errors.Wrap(err, "cannot obtain memory info")
	}
	return int(memInfo.Total/memPerTab)*2 + 1, nil
}

// openTabs opens tabs to create memory pressure.
func openTabs(ctx context.Context, s *testing.State, cr *chrome.Chrome, createTabCount int, mbPerTab int64, baseURL string) error {
	for i := 0; i < createTabCount; i++ {
		url := allocationURL(baseURL, mbPerTab, compressRaio, int64(i))
		if err := openAllocationPage(ctx, url, cr); err != nil {
			return errors.Wrap(err, "cannot create tab")
		}

		// Waits extra 2 seconds.
		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			return errors.Wrap(err, "Sleep timeout")
		}
	}
	return nil
}

// activeTabURL returns the URL of the acitve tab.
func activeTabURL(ctx context.Context, cr *chrome.Chrome) (string, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "cannot create test connection")
	}

	var tabURL string
	const exp = `
		new Promise((resolve, reject) => {
			chrome.tabs.query({active: true}, (tlist) => {
				resolve(tlist[0].url);
			});
		});`
	if err := tconn.EvalPromise(ctx, exp, &tabURL); err != nil {
		return "", errors.Wrap(err, "EvalPromise failed")
	}
	return tabURL, nil
}

// reloadActiveTab reloads the active tab.
func reloadActiveTab(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test connection")
	}

	const exp = "chrome.tabs.reload();"
	if err := tconn.Eval(ctx, exp, nil); err != nil {
		return errors.Wrap(err, "Eval failed")
	}
	return nil
}

// reloadCrashedTab reload the active tab if it's crashed. Returns whether the tab is reloaded.
func reloadCrashedTab(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	tabURL, err := activeTabURL(ctx, cr)
	if err != nil {
		return false, errors.Wrap(err, "cannot get active tab URL")
	}

	// If the active tab's URL is not in the devtools targets, the active tab is crashed.
	if !cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(tabURL)) {
		testing.ContextLog(ctx, "Reload tab:", tabURL)
		if err := reloadActiveTab(ctx, cr); err != nil {
			return false, errors.Wrap(err, "reloadActiveTab failed")
		}
		if err := waitAllocationForURL(ctx, cr, tabURL); err != nil {
			return false, errors.Wrap(err, "waitAllocationForURL failed")
		}
		return true, nil
	}
	return false, nil
}

// switchTabs switches between tabs and reload crashed tabs. Returns the reload cound.
func switchTabs(ctx context.Context, s *testing.State, cr *chrome.Chrome, switchCount int) (int, error) {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cannot initialize keyboard")
	}

	waitTimeMs := 3000
	reloadCount := 0
	for i := 0; i < switchCount; i++ {
		if err := ew.Accel(ctx, "ctrl+tab"); err != nil {
			return 0, errors.Wrap(err, "Accel(Ctrl+Tab) failed")
		}
		// Waits between tab switches.
		// +/- within 1000ms from the previous wait time to cluster long and short wait times.
		// On some settings, it's easier to trigger OOM with clustered short wait times.
		// On other settings, it's easier with clustered long wait times.
		waitTimeMs += rand.Intn(2001) - 1000
		if waitTimeMs < 1000 {
			waitTimeMs = 1000
		} else if waitTimeMs > 5000 {
			waitTimeMs = 5000
		}
		if err := testing.Sleep(ctx, time.Duration(waitTimeMs)*time.Millisecond); err != nil {
			return 0, errors.Wrap(err, "Sleep timeout")
		}
		testing.ContextLogf(ctx, "%3d, wait ms: %d", i, waitTimeMs)

		reloaded, err := reloadCrashedTab(ctx, cr)
		if err != nil {
			return 0, errors.Wrap(err, "reloadCrashedTab failed")
		}
		if reloaded {
			reloadCount++
		}
	}
	return reloadCount, nil
}
