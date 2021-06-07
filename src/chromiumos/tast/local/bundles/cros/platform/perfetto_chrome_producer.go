// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
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
		Attr:         []string{"group:mainline"},
	})
}

// ensureJobsStopped stops traced and makes sure traced_probes is also stopped.
func ensureJobsStopped1(ctx context.Context) error {
	const (
		tracedJob    = "traced"
		waitDuration = 10 * time.Second
	)

	// Use a short context for waiting job status.
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()

	// Stop traced.
	if err := upstart.StopJob(wctx, tracedJob); err != nil {
		return errors.Wrap(err, "failed to stop the traced job")
	}

	// Check that traced_probes is also stopped with traced.
	if err := upstart.WaitForJobStatus(wctx, tracedJob, upstartcommon.StopGoal, upstartcommon.WaitingState, upstart.RejectWrongGoal, waitDuration); err != nil {
		return errors.Wrap(err, "the traced job isn't stopped")
	}

	return nil
}

func waitForChromeProducer(ctx context.Context) error {
	return testing.Poll(ctx, func(context.Context) error {
		cmd := testexec.CommandContext(ctx, "/usr/bin/perfetto", "--query")

		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to run the tracing session")
		}

		chromium := 0
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "name: \"org.chromium.trace_metadata\"") {
				chromium++
			}
		}
		if chromium < 1 {
			return errors.Errorf("unexpected number (%d) of chromium producer connected", chromium)
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  20 * time.Second,
		Interval: 500 * time.Millisecond,
	})
	return nil
}

// PerfettoChromeProducer tests Chrome as a trace producer.
func PerfettoChromeProducer(ctx context.Context, s *testing.State) {
	wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
	defer wcancel()
	// Make sure traced is running (and start it if not).
	if err := upstart.EnsureJobRunning(wctx, "traced"); err != nil {
		s.Fatalf("Job traced isn't running")
	}
	defer func() {
		if err := ensureJobsStopped1(ctx); err != nil {
			s.Fatal("Error in stopping the jobs: ", err)
		}
	}()

	// Start Chrome with the "EnablePerfettoSystemTracing" feature flag.
	cr, err := chrome.New(
		ctx,
		chrome.ExtraArgs("--enable-features=EnablePerfettoSystemTracing"))
	if err != nil {
		s.Fatalf("Failed to enable Perfetto system tracing for Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Run 'perfetto --config' and check whether Chrome producers are in the list.
	wctx, wcancel = context.WithTimeout(ctx, 20*time.Second)
	defer wcancel()
	if err = waitForChromeProducer(wctx); err != nil {
		s.Fatalf("Chrome producer not connected: ", err)
	}
}
