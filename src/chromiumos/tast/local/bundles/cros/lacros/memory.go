// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Memory,
		Desc:         "Tests lacros memory usage",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"disabled"},
		Data: []string{
			launcher.DataArtifact,
		},
		Pre:     launcher.StartedByData(),
		Timeout: 60 * time.Minute,
		Params: []testing.Param{{
			Name: "blank",
			Val:  "about:blank",
		}, {
			Name: "docs",
			Val:  "https://docs.google.com/document/d/1_WmgE1F5WUrhwkPqJis3dWyOiUmQKvpXp5cd4w86TvA/edit",
		}, {
			Name: "reddit",
			Val:  "https://old.reddit.com/",
		},
		},
	})
}

// findMatch looks for lines of the form `[stat]:  123 kB` and sums the
// numerical values, returning the output in bytes.
func findMatch(input []byte, stat string) (int, error) {
	re := regexp.MustCompile(stat + `:\s*(\d*)\s*kB`)
	results := re.FindAllSubmatch(input, -1)
	sum := 0
	for _, result := range results {
		n, err := strconv.Atoi(string(result[1]))
		if err != nil {
			return 0, err
		}
		sum += n * 1024
	}
	return sum, nil
}

// procSum is a complex function.
//  1. It finds all processes whose command line includes |path|.
//  2. It queries /proc/{pid}/{endpoint} for each process.
//  3. It filters and sums across all statistics that match |stat|.
func procSum(ctx context.Context, path string, endpoint string, stat string) (int, error) {
	pids, err := launcher.PidsFromPath(ctx, path)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get pids for "+path)
	}

	var total = 0
	for _, pid := range pids {
		// Query /proc. Ignore errors reading the file because the
		// process may no longer exist.
		content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, endpoint))
		if err == nil {
			value, err := findMatch(content, stat)
			if err != nil {
				return 0, errors.Wrap(err, "failed to find match")
			}
			total += value
		}
	}

	return total, nil
}

// measureProcesses returns memory estimates for all processes that contain a path
// in their command line. The first int is (RssAnon + VmSwap). This is Chrome's
// definition of PrivateMemoryFootprint, and serves as an underestimate of
// memory usage. The second int is (Pss). This is an overestimate of memory
// usage.
func measureProcesses(ctx context.Context, path string) (int, int, error) {
	j, err := procSum(ctx, path, "status", "RssAnon")
	if err != nil {
		return 0, 0, err
	}
	k, err := procSum(ctx, path, "status", "VmSwap")
	if err != nil {
		return 0, 0, err
	}
	p, err := procSum(ctx, path, "smaps", "Pss")
	if err != nil {
		return 0, 0, err
	}
	return j + k, p, nil
}

// measureBothChrome measures the current memory usage of both linux-chrome and
// chromeos-chrome. Returns (pmf, pss) in bytes.
func measureBothChrome(ctx context.Context, s *testing.State) (int, int) {
	// As a rule of thumb, we wait 60 seconds before taking any
	// measurements. This gives time for previous operations to finish and
	// the system to quiesce. In particular, both linux-chrome and
	// chromeos-chrome will sometimes spawn/keep around unnecessary
	// processes, but most will go away after 60 seconds.
	testing.Sleep(ctx, 60*time.Second)

	pmf, pss, err := measureProcesses(ctx, launcher.BinaryPath)
	if err != nil {
		s.Fatal("Failed to measure memory of linux-chrome: ", err)
	}
	chromeosChromePath := "/opt/google/chrome"
	pmf1, pss1, err := measureProcesses(ctx, chromeosChromePath)
	if err != nil {
		s.Fatal("Failed to measure memory of chromeos-chrome: ", err)
	}
	return pmf + pmf1, pss + pss1
}

// Memory is a basic test for lacros memory usage. It measures the PMF and PSS
// overhead for linux-chrome with a single about:blank tab. It also makes the
// same measurements for chromeos-chrome. This estimate is not perfect. For
// example, this test does not measure the size of the chromeos-chrome test API
// extension, but it does include the extension for linux-chrome.
// Furthermore, this test does not have fine control over chromeos-chrome,
// which may choose to spawn/kill utility or renderer processes for its own
// purposes. My running the same code 10 times, outliers become obvious.
func Memory(ctx context.Context, s *testing.State) {
	url := s.Param().(string)
	for i := 0; i < 10; i++ {
		// Measure memory before launching linux-chrome.
		pmf1, pss1 := measureBothChrome(ctx, s)

		// We currently rely on the assumption that the launcher
		// creates a windows that is 800x600 in size.
		l, err := launcher.LaunchLinuxChrome(ctx, s.PreValue().(launcher.PreData))
		if err != nil {
			s.Fatal("Failed to launch linux-chrome: ", err)
		}

		// Open a new tab and navigate to |url|.
		newTab, err := l.Devsess.CreateTarget(ctx, url)
		if err != nil {
			s.Fatal("Failed to open new tab: ", err)
		}

		// Close the initial "about:blank" tab present at startup.
		targetFilter := func(t *target.Info) bool {
			return t.URL == "about:blank"
		}
		targets, err := l.Devsess.FindTargets(ctx, targetFilter)
		if err != nil {
			s.Fatal("Failed to query for about:blank pages: ", err)
		}
		for _, info := range targets {
			if target := info.TargetID; target != newTab {
				l.Devsess.CloseTarget(ctx, target)
			}
		}

		// Measure memory after launching linux-chrome.
		pmf2, pss2 := measureBothChrome(ctx, s)
		testing.ContextLogf(ctx, "linux-chrome RssAnon + VmSwap (MB): %v. Pss (MB): %v ", (pmf2-pmf1)/1024/1024, (pss2-pss1)/1024/1024)

		// Close linux-chrome
		l.Close(ctx)

		// Measure memory before launching chromeos-chrome.
		pmf3, pss3 := measureBothChrome(ctx, s)

		// Open a new tab to |url|.
		conn, err := s.PreValue().(launcher.PreData).Chrome.NewConn(ctx, url)
		if err != nil {
			s.Fatal("Failed to open chromeos-chrome tab: ", err)
		}
		defer conn.Close()

		// Set the window to 800x600 in size.
		err = s.PreValue().(launcher.PreData).TestAPIConn.EvalPromise(ctx,
			`new Promise((resolve, reject) => {
			  chrome.windows.getLastFocused({}, (window) => {
				  chrome.windows.update(window.id, {width: 800, height:600, state:"normal"}, resolve);
			  });
			})
			`, nil)
		if err != nil {
			s.Fatal("Setting window size failed: ", err)
		}

		// Measure memory after launching chromeos-chrome.
		pmf4, pss4 := measureBothChrome(ctx, s)
		testing.ContextLogf(ctx, "chromeos-chrome RssAnon + VmSwap (MB): %v. Pss (MB): %v ", (pmf4-pmf3)/1024/1024, (pss4-pss3)/1024/1024)

		// Close chromeos-chrome
		conn.CloseTarget(ctx)
	}
}
