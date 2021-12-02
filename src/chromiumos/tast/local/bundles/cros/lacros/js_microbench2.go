// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         JSMicrobench2,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Runs JS microbenasdfsdfch against both ash-chrome and lacros-chrome",
		Contacts:     []string{"hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		// Waiting for the stability can take longer time. So, we have a longer buffer.
		Timeout: 6 * time.Minute,
	})
}

// jsMicrobenchCode is the core JS code snippet to measure the JS performance
// between ash-chrome and lacros-chrome. Shared between cdp-based testing and
// HTML based testing.
const jsMicrobenchCode = `
  let elapsed, ignored;
  eval(` + "`" + `
    let start = performance.now();
    let sum = 0;
    for (let i = 0; i < 1000000000; i++) {
      sum += i;
    }
    let end = performance.now();
    elapsed = end - start;
    ignored = sum;
  ` + "`" + `);`

func JSMicrobench(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(launcher.FixtValue)

	cleanup, err := lacros.SetupPerfTest(ctx, f.TestAPIConn(), "lacros.JSMicrobench")
	if err != nil {
		s.Fatal("Failed to set up lacros perf test: ", err)
	}
	defer cleanup(ctx)

	// Prepare HTML data file.
	dir, err := ioutil.TempDir("/home/chronos/user/Downloads", "")
	if err != nil {
		s.Fatal("Failed to create working directory: ", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			s.Logf("Failed to remove working dir at %q: %v", dir, err)
		}
	}()

	if err := os.Chmod(dir, 0755); err != nil {
		s.Fatal("Failed to set permission to the working directory: ", err)
	}

	htmlPath, err := createJSMicrobenchHTML(ctx, dir)
	if err != nil {
		s.Fatal("Failed to create micorbench html: ", err)
	}
	defer os.Remove(htmlPath)

	pv := perf.NewValues()

	// Run JS benchmark against ash-chrome.
	if elapsed, err := runJSMicrobench(ctx, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		return lacros.SetupCrosTestWithPage(ctx, f, url, lacros.StabilizeAfterOpeningURL)
	}); err != nil {
		s.Error("Failed to run ash-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "jsmicrobench_ash",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, elapsed.Seconds())
	}

	// Run JS benchmark against lacros-chrome.
	if elapsed, err := runJSMicrobench(ctx, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		conn, _, _, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, f, url, lacros.StabilizeAfterOpeningURL)
		return conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run lacros-chrome benchrmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "jsmicrobench_lacros",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, elapsed.Seconds())
	}

	// Run JS benchmark against ash-chrome.
	if elapsed, err := runJSMicrobenchFromHTML(ctx, htmlPath, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		return lacros.SetupCrosTestWithPage(ctx, f, url, lacros.StabilizeAfterOpeningURL)
	}); err != nil {
		s.Error("Failed to run ash-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "jsmicrobench_html_ash",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, elapsed.Seconds())
	}

	// Run JS benchmark against lacros-chrome.
	if elapsed, err := runJSMicrobenchFromHTML(ctx, htmlPath, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		conn, _, _, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, f, url, lacros.StabilizeAfterOpeningURL)
		return conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run lacros-chrome benchrmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "jsmicrobench_html_lacros",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, elapsed.Seconds())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

// runJSMicrobench runs very simple javascript benchmark.
// setup should prepare the browser (ash-chrome or lacros-chrome) with opening a given URL in a new tab
// and returns the connection to it.
func runJSMicrobench(
	ctx context.Context,
	setup func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error)) (time.Duration, error) {
	conn, cleanup, err := setup(ctx, chrome.BlankURL)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open a new tab")
	}
	defer cleanup(ctx)

	var result struct {
		Elapsed float64 `json:"elapsed"`
		// Ignored is the result of the calculation. Accept here to avoid opitmized out in JS code.
		Ignored float64 `json:"ignored"`
	}
	if err := conn.Eval(ctx, `(() => {`+jsMicrobenchCode+`
	  return {"elapsed": elapsed, "ignored": ignored};
	})()`, &result); err != nil {
		return 0, errors.Wrap(err, "failed to run JS microbenchmark")
	}
	return time.Duration(result.Elapsed * float64(time.Millisecond)), nil
}

// runJSMicrobenchFromHTML runs the same microbenchmark as runJSMicrobench.
// The difference is, instead of running the benchmark directly on CDP connection,
// this test loads the HTML page with the same microbenchmark code.
func runJSMicrobenchFromHTML(
	ctx context.Context,
	path string,
	setup func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error)) (time.Duration, error) {
	conn, cleanup, err := setup(ctx, chrome.BlankURL)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open a new tab")
	}
	defer cleanup(ctx)

	// Navigate the blankpage to the HTML file to be loaded.
	// This blocks until the loading is completed. Specifically in this case,
	// it blocks if the microbenchmark is running.
	if err := conn.Navigate(ctx, "file://"+path); err != nil {
		return 0, errors.Wrap(err, "failed to navigate a blankpage to the path")
	}

	var elapsed float64
	if err := conn.Eval(ctx, "elapsed", &elapsed); err != nil {
		return 0, errors.Wrap(err, "failed to run JS microbenchmark")
	}
	return time.Duration(elapsed * float64(time.Millisecond)), nil
}

// createJSMicrobenchHTML creates a temporary file at dir to be loaded for
// runJSMicrobenchFromHTML.
// Callers have the responsibility to remove the file after its use.
func createJSMicrobenchHTML(ctx context.Context, dir string) (string, error) {
	const content = `
<p id="p">Running...</p>
<script>` + jsMicrobenchCode + `

let paragraph = document.getElementById("p");
paragraph.appendChild(document.createTextNode(elapsed));

let result = "\nignore me " + ignored % 1000;
paragraph.appendChild(document.createTextNode(result))
</script>`

	path := filepath.Join(dir, "microbench.html")
	if err := ioutil.WriteFile(path, []byte(content), 0666); err != nil {
		return "", errors.Wrap(err, "failed to create microbench.html")
	}
	return path, nil
}
