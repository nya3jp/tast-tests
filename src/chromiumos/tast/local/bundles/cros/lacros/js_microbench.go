// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
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
		Func:         JSMicrobench,
		Desc:         "Runs JS micorbench against both ash-chrome and lacros-chrome",
		Contacts:     []string{"hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		// Waiting for the stability can take longer time. So, we have a longer buffer.
		Timeout: 4 * time.Minute,
	})
}

func JSMicrobench(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(launcher.FixtValue)

	cleanup, err := lacros.SetupPerfTest(ctx, f.TestAPIConn(), "lacros.JSMicorbench")
	if err != nil {
		s.Fatal("Failed to set up lacros perf test: ", err)
	}
	defer cleanup(ctx)

	pv := perf.NewValues()

	// Run JS benchmark against ash-chrome.
	if elapsed, err := runJSMicrobench(ctx, func(ctx context.Context, url string) (*chrome.Conn, lacros.CleanupCallback, error) {
		return lacros.SetupCrosTestWithPage(ctx, f, url)
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
		conn, _, _, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, f, url)
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
	if err := conn.Eval(ctx, `(() => {
	  let elapsed, ignored;
	  eval(`+"`"+`
	    let start = performance.now();
	    let sum = 0;
	    for (let i = 0; i < 1000000000; i++) {
	      sum += i;
	    }
	    let end = performance.now();
	    elapsed = end - start;
	    ignored = sum;
	  `+"`"+`);
	  return {"elapsed": elapsed, "ignored": ignored};
	})()`, &result); err != nil {
		return 0, errors.Wrap(err, "failed to run JS microbenchmark")
	}
	return time.Duration(result.Elapsed * float64(time.Millisecond)), nil
}
