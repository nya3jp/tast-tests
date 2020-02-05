// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/lacros/launcher"
	"chromiumos/tast/local/testexec"
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
		Timeout: 30 * time.Minute,
	})
}

// findMatch looks for lines of the form `[stat]:  123 kB` and sums the
// numerical values.
func findMatch(input []byte, stat string) (int, error) {
	re := regexp.MustCompile(stat + `:\s*(\d*)\s*kB`)
	results := re.FindAllSubmatch(input, -1)
	sum := 0
	for _, result := range results {
		n, err := strconv.Atoi(string(result[1]))
		if err != nil {
			return 0, err
		}
		sum += n
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

	var pidsString = []string{}
	for _, pid := range pids {
		pidsString = append(pidsString, strconv.Itoa(pid))
	}

	// Query /proc.
	var total = 0
	for _, pid := range pidsString {
		statusCmd := testexec.CommandContext(ctx, "cat", "/proc/"+pid+"/"+endpoint)
		a, err := statusCmd.Output()
		if err == nil {
			value, err := findMatch(a, stat)
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
// chromeos-chrome. Returns (pmf, pss) in MB.
func measureBothChrome(ctx context.Context, s *testing.State) (int, int) {
	pmf, pss, err := measureProcesses(ctx, launcher.BinaryPath)
	if err != nil {
		s.Fatal("Failed to measure memory of linux-chrome: ", err)
	}
	chromeosChromePath := "/opt/google/chrome"
	pmf1, pss1, err := measureProcesses(ctx, chromeosChromePath)
	if err != nil {
		s.Fatal("Failed to measure memory of chromeos-chrome: ", err)
	}
	return (pmf + pmf1) / 1024, (pss + pss1) / 1024
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
	for i := 0; i < 10; i++ {
		pmf1, pss1 := measureBothChrome(ctx, s)

		// We currently rely on the assumption that the launcher
		// creates a windows that is 800x600 in size.
		l, err := launcher.LaunchLinuxChrome(ctx, s.PreValue().(launcher.PreData))
		if err != nil {
			s.Fatal("Failed to launch linux-chrome: ", err)
		}
		pmf2, pss2 := measureBothChrome(ctx, s)

		testing.ContextLogf(ctx, "linux-chrome RssAnon + VmSwap (MB): %v. Pss (MB): %v ", pmf2-pmf1, pss2-pss1)
		l.Close(ctx)

		pmf3, pss3 := measureBothChrome(ctx, s)

		conn, err := s.PreValue().(launcher.PreData).Chrome.NewConn(ctx, "about:blank")
		if err != nil {
			s.Fatal("Failed to open chromeos-chrome tab: ", err)
		}

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

		pmf4, pss4 := measureBothChrome(ctx, s)
		testing.ContextLogf(ctx, "chromeos-chrome RssAnon + VmSwap (MB): %v. Pss (MB): %v ", pmf4-pmf3, pss4-pss3)

		// We have no guarantee that the renderer has actually been
		// shut down. Wait 5 seconds to be safe.
		conn.CloseTarget(ctx)
		testing.Sleep(5 * time.Second)
	}
}
