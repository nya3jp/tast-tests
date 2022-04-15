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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

type testMode int

const (
	openURLMode testMode = iota
	openTabMode
)

type testParams struct {
	mode    testMode
	url     string
	numTabs int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Memory,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests lacros memory usage",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "lacros",
		Timeout:      60 * time.Minute,
		Params: []testing.Param{{
			Name: "blank",
			Val:  testParams{mode: openURLMode, url: "about:blank"},
		}, {
			Name: "docs",
			Val:  testParams{mode: openURLMode, url: "https://docs.google.com/document/d/1_WmgE1F5WUrhwkPqJis3dWyOiUmQKvpXp5cd4w86TvA/edit"},
		}, {
			Name: "reddit",
			Val:  testParams{mode: openURLMode, url: "https://old.reddit.com/"},
		}, {
			Name: "youtube",
			Val:  testParams{mode: openURLMode, url: "https://www.youtube.com/watch?v=uS33jC2VYNU"},
		}, {
			Name: "twentytabs",
			Val:  testParams{mode: openTabMode, numTabs: 20},
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
//  1. It finds all processes whose command line includes path.
//  2. It queries /proc/{pid}/{endpoint} for each process.
//  3. It filters and sums across all statistics that match stat.
func procSum(ctx context.Context, path, endpoint, stat string) (int, error) {
	pids, err := lacros.PidsFromPath(ctx, path)
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

// measureBothChrome measures the current memory usage of both lacros-chrome and
// ash-chrome. Returns (pmf, pss) in bytes.
func measureBothChrome(ctx context.Context, s *testing.State) (int, int) {
	// As a rule of thumb, we wait 60 seconds before taking any
	// measurements. This gives time for previous operations to finish and
	// the system to quiesce. In particular, both lacros-chrome and
	// ash-chrome will sometimes spawn/keep around unnecessary
	// processes, but most will go away after 60 seconds.
	testing.Sleep(ctx, 60*time.Second)

	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	info, err := lacros.InfoSnapshot(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get lacros info: ", err)
	}

	pmf, pss, err := measureProcesses(ctx, info.LacrosPath)
	if err != nil {
		s.Fatal("Failed to measure memory of lacros-chrome: ", err)
	}
	chromeosChromePath := "/opt/google/chrome"
	pmf1, pss1, err := measureProcesses(ctx, chromeosChromePath)
	if err != nil {
		s.Fatal("Failed to measure memory of ash-chrome: ", err)
	}
	return pmf + pmf1, pss + pss1
}

// setWindowSize sets the last focused window to 800x600 in size. For lacros
// windows, use the lacros TestConn. For ash, use the ash TestConn.
func setWindowSize(ctx context.Context, tconn *chrome.TestConn) error {
	// Set the window to 800x600 in size.
	if err := tconn.Call(ctx, nil, `async () => {
		const win = await tast.promisify(chrome.windows.getLastFocused)();
		await tast.promisify(chrome.windows.update)(win.id, {width: 800, height:600, state:"normal"});
	}`); err != nil {
		return errors.Wrap(err, "setting window size failed")
	}

	return nil
}

// Memory is a basic test for lacros memory usage. It measures the PMF and PSS
// overhead for lacros-chrome with a single about:blank tab. It also makes the
// same measurements for ash-chrome. This estimate is not perfect. For
// example, this test does not measure the size of the ash-chrome test API
// extension, but it does include the extension for lacros-chrome.
// Furthermore, this test does not have fine control over ash-chrome,
// which may choose to spawn/kill utility or renderer processes for its own
// purposes. My running the same code 10 times, outliers become obvious.
func Memory(ctx context.Context, s *testing.State) {
	params := s.Param().(testParams)
	url := params.url
	for i := 0; i < 10; i++ {
		// Measure memory before launching lacros-chrome.
		pmf1, pss1 := measureBothChrome(ctx, s)

		tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to test API: ", err)
		}

		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros-chrome: ", err)
		}

		ltconn, err := l.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get lacros-chrome TestConn: ", err)
		}

		if err := setWindowSize(ctx, ltconn); err != nil {
			s.Fatal("Failed to set lacros-chrome window size: ", err)
		}

		if params.mode == openTabMode {
			if err := openTabsLacros(ctx, l, params.numTabs); err != nil {
				s.Fatal("Failed to oepn lacros-chrome tabs: ", err)
			}
		} else {
			if err := navigateSingleTabToURLLacros(ctx, url, l); err != nil {
				s.Fatal("Failed to open a lacros tab: ", err)
			}
		}

		// Measure memory after launching lacros-chrome.
		pmf2, pss2 := measureBothChrome(ctx, s)
		testing.ContextLogf(ctx, "lacros-chrome RssAnon + VmSwap (MB): %v. Pss (MB): %v ", (pmf2-pmf1)/1024/1024, (pss2-pss1)/1024/1024)

		// Close lacros-chrome
		l.Close(ctx)

		// Measure memory before launching ash-chrome.
		pmf3, pss3 := measureBothChrome(ctx, s)

		var conns []*chrome.Conn
		if params.mode == openTabMode {
			conns, err = openTabsChromeOS(ctx, s.FixtValue().(chrome.HasChrome).Chrome(), params.numTabs)
			if err != nil {
				s.Fatal("Failed to open ash-chrome tabs: ", err)
			}
		} else {
			// Open a new tab to url.
			conn, err := s.FixtValue().(chrome.HasChrome).Chrome().NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to open ash-chrome tab: ", err)
			}
			conns = append(conns, conn)
		}
		for _, conn := range conns {
			defer conn.Close()
		}

		if err := setWindowSize(ctx, tconn); err != nil {
			s.Fatal("Failed to set lacros-chrome window size: ", err)
		}

		// Measure memory after launching ash-chrome.
		pmf4, pss4 := measureBothChrome(ctx, s)
		testing.ContextLogf(ctx, "ash-chrome RssAnon + VmSwap (MB): %v. Pss (MB): %v ", (pmf4-pmf3)/1024/1024, (pss4-pss3)/1024/1024)

		// Close ash-chrome
		for _, conn := range conns {
			conn.CloseTarget(ctx)
		}
	}
}

// navigateSingleTabToURLLacros assumes that there's a freshly launched instance
// of lacros-chrome, with a single tab open to about:blank, then, navigates the
// blank tab to the given url.
func navigateSingleTabToURLLacros(ctx context.Context, url string, l *lacros.Lacros) error {
	// Open a new tab and navigate to url.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("about:blank"))
	if err != nil {
		return errors.Wrap(err, "failed to find an about:blank tab")
	}
	defer conn.Close()
	if err := conn.Navigate(ctx, url); err != nil {
		return errors.Wrapf(err, "failed to navigate to %q", url)
	}
	return nil
}

// openTabsLacros assumes that lacros-chrome has been freshly launched,
// with a single tab opened to about:blank.
func openTabsLacros(ctx context.Context, l *lacros.Lacros, numTabs int) error {
	for i := 0; i < numTabs-1; i++ {
		// Open a new tab and navigate to about blank
		conn, err := l.NewConn(ctx, "about:blank")
		if err != nil {
			return err
		}
		if err := conn.Close(); err != nil {
			return err
		}

		// Wait one second to quiesce.
		testing.Sleep(ctx, time.Second)
	}
	return nil
}

// openTabsChromeOS assumes that ash-chrome is running, but that
// there is no open window.
func openTabsChromeOS(ctx context.Context, c *chrome.Chrome, numTabs int) ([]*chrome.Conn, error) {
	var conns []*chrome.Conn
	for i := 0; i < numTabs; i++ {
		conn, err := c.NewConn(ctx, "about:blank")
		if err != nil {
			for _, conn := range conns {
				conn.Close()
			}
			return nil, err
		}
		conns = append(conns, conn)
	}
	return conns, nil
}
