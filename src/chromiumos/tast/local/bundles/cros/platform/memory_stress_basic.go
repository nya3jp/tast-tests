// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
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

func getAllocURL(baseURL string, sizeMB int64, randomRatio float64, id int64) string {
	url := baseURL
	url += "?alloc=" + strconv.FormatInt(sizeMB, 10)
	url += "&ratio=" + strconv.FormatFloat(randomRatio, 'f', 3, 64)
	url += "&id=" + strconv.FormatInt(id, 10)
	return url
}

// waitAllocation waits for completion of javascript memory allocation.
func waitAllocation(ctx context.Context, conn *chrome.Conn) error {
	timeout := 60 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Waits for completion of javascript allocation.
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

// waitAllocationForURL waits for completion of javascript memory allocation
// on the tab with specified url.
func waitAllocationForURL(ctx context.Context, cr *chrome.Chrome, url string) {
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		testing.ContextLog(ctx, "NewConnForTarget failed: ", err)
		return
	}
	defer conn.Close()
	if err := waitAllocation(ctx, conn); err != nil {
		testing.ContextLog(ctx, "waiting allocation failed: ", err)
	}
}

// openAllocPage opens a page to alloc a lot of javascript objects.
func openAllocPage(ctx context.Context, url string, cr *chrome.Chrome) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new renderer")
	}

	if err := waitAllocation(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// getOpenTabCount returns the count of tabs to open.
func getOpenTabCount(s *testing.State, mbPerTab int) int {
	memPerTab := kernelmeter.NewMemSizeMiB(mbPerTab)
	memInfo, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Cannot obtain memory info: ", err)
	}
	return int(memInfo.Total/memPerTab)*2 + 1
}

// openTabs opens tabs to create memory pressure.
func openTabs(ctx context.Context, s *testing.State, cr *chrome.Chrome, createTabCount int, mbPerTab int64, baseURL string) []*chrome.Conn {
	var conns []*chrome.Conn
	for i := 0; i < createTabCount; i++ {
		url := getAllocURL(baseURL, mbPerTab, compressRaio, int64(i))
		conn, err := openAllocPage(ctx, url, cr)
		if err != nil {
			s.Fatal("Cannot create tab")
		}
		conns = append(conns, conn)

		// Waits extra 2 seconds.
		if err := testing.Sleep(ctx, 2*time.Second); err != nil {
			s.Fatal("sleep timeout: ", err)
		}
	}
	return conns
}

// getActiveTabURL returns the url of the acitve tab.
func getActiveTabURL(ctx context.Context, cr *chrome.Chrome) (string, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Cannot create test connection: ", err)
	}

	var tabURL string
	query := "chrome.tabs.query({active: true}, (tlist) => { resolve(tlist[0].url) })"
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", query)
	if err := tconn.EvalPromise(ctx, promise, &tabURL); err != nil {
		testing.ContextLog(ctx, "eval err: ", err)
		return "", errors.Wrap(err, "cannot get tab url")
	}
	return tabURL, nil
}

// switchTabs switches between tabs and reload crashed tabs. Returns the reload cound.
func switchTabs(ctx context.Context, s *testing.State, cr *chrome.Chrome, switchCount int) int {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Cannot initialize keyboard")
	}

	waitTimeMs := 3000
	reloadCount := 0
	for i := 0; i < switchCount; i++ {
		if err := ew.Accel(ctx, "ctrl+tab"); err != nil {
			s.Fatal("Accel(Ctrl+Tab) returned error")
		}
		// Waits between tab switches.
		waitTimeMs += rand.Intn(2001) - 1000
		if waitTimeMs < 1000 {
			waitTimeMs = 1000
		}
		if waitTimeMs > 5000 {
			waitTimeMs = 5000
		}
		if err := testing.Sleep(ctx, time.Duration(waitTimeMs)*time.Millisecond); err != nil {
			s.Fatal("sleep timeout: ", err)
		}
		testing.ContextLogf(ctx, "%3d, wait ms: %d", i, waitTimeMs)

		// Reloads the tab if it's crashed.
		// If the active tab's url is not in the devtools targets, the
		// active tab is crashed.
		tabURL, err := getActiveTabURL(ctx, cr)
		if err != nil {
			testing.ContextLog(ctx, "Cannot get tab url: ", err)
		}
		if !cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(tabURL)) {
			reloadCount++
			testing.ContextLog(ctx, "reload tab:", tabURL)
			if err := ew.Accel(ctx, "ctrl+r"); err != nil {
				s.Fatal("Accel(Ctrl+r) returned error")
			}
			waitAllocationForURL(ctx, cr, tabURL)
		}
	}
	return reloadCount
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
	testing.ContextLog(ctx, "debug addr: ", cr.DebugAddrPort())

	createTabCount := getOpenTabCount(s, mbPerTab)
	testing.ContextLog(ctx, "Tab count to create: ", createTabCount)

	url := server.URL + "/" + allocPageFilename

	conns := openTabs(ctx, s, cr, createTabCount, mbPerTab, url)
	reloadCount := switchTabs(ctx, s, cr, switchCount)

	reloadTabMetric := perf.Metric{
		Name:      "tast_reload_tab_count",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(reloadTabMetric, float64(reloadCount))
	testing.ContextLog(ctx, "reload tab count:", reloadCount)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}

	for _, conn := range conns {
		conn.Close()
	}
	cr.Close(ctx)
}
