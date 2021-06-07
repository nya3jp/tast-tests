// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoChromeProducer,
		Desc:         "Tests Chrome connecting to the Perfetto system tracing service",
		Contacts:     []string{"chinglinyu@chromium.org", "chenghaoyang@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"}, // TODO(b/190474394) remote "information" after the test is stable.
	})
}

// stopTraced stops the upstart job "traced".
func stopTraced(ctx context.Context) error {
	const (
		tracedJob    = "traced"
		waitDuration = 10 * time.Second
	)

	// Use a short context for waiting job status.
	wctx, wcancel := context.WithTimeout(ctx, waitDuration)
	defer wcancel()

	// Stop traced.
	if err := upstart.StopJob(wctx, tracedJob); err != nil {
		return err
	}

	return nil
}

// waitForChromeProducer waits until the required number of Chrome producers are connected to traced.
func waitForChromeProducer(ctx context.Context) error {
	const (
		requiredChromeProducers = 2
	)

	return testing.Poll(ctx, func(context.Context) error {
		cmd := testexec.CommandContext(ctx, "/usr/bin/perfetto", "--query")

		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to query the service state of traced")
		}

		// Count the number of data sources named "org.chromium.trace_event".
		chromeProducers := 0
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "name: \"org.chromium.trace_event\"") {
				chromeProducers++
			}
		}
		if chromeProducers < requiredChromeProducers {
			return errors.Errorf("unexpected number (%d) of Chrome producer connected", chromeProducers)
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  20 * time.Second,
		Interval: 500 * time.Millisecond,
	})
	return nil
}

// PerfettoChromeProducer tests Chrome as a perfetto trace producer.
// The test enables the "EnablePerfettoSystemTracing" feature flag for Chrome and then checks if traced sees multiple Chrome producers connected (browser, renderer, utility, etc.).
func PerfettoChromeProducer(ctx context.Context, s *testing.State) {
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()
	// Make sure traced is running (and start it if not).
	if err := upstart.EnsureJobRunning(wctx, "traced"); err != nil {
		s.Fatal("Job traced isn't running: ", err)
	}
	defer func() {
		if err := stopTraced(ctx); err != nil {
			s.Fatal("Error in stopping traced: ", err)
		}
	}()

	// Start Chrome with the "EnablePerfettoSystemTracing" feature flag.
	cr, err := chrome.New(
		ctx,
		chrome.ExtraArgs("--enable-features=EnablePerfettoSystemTracing"))
	if err != nil {
		s.Fatal("Failed to enable Perfetto system tracing for Chrome: ", err)
	}
	defer func() {
		cr.Close(ctx)
		upstart.RestartJob(ctx, "ui") // Restart ui to reset the feature flag.
	}()

	wctx, wcancel = context.WithTimeout(ctx, 20*time.Second)
	defer wcancel()
	if err = waitForChromeProducer(wctx); err != nil {
		s.Fatal("Failed in waiting for Chrome producers: ", err)
	}
}
