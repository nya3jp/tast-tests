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
	"chromiumos/tast/local/bundles/cros/platform/perfetto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoChromeProducer,
		Desc:         "Tests Chrome connecting to the Perfetto system tracing service",
		Contacts:     []string{"chinglinyu@chromium.org", "chenghaoyang@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Attr:         []string{"group:mainline"},
	})
}

// waitForChromeProducer waits until the required number of Chrome producers are connected to the system tracing service daemon.
func waitForChromeProducer(ctx context.Context) error {
	// At least 2 Chrome producers (browser, renderer, utility, etc).
	const requiredChromeProducers = 2

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
		// Chrome producers retry the connection with delay on failure to connect. Poll using a 30 second timeout.
		Timeout: 30 * time.Second,
	})
}

// PerfettoChromeProducer tests Chrome as a perfetto trace producer.
// The test enables the "EnablePerfettoSystemTracing" feature flag for Chrome and then checks if traced sees multiple Chrome producers connected.
func PerfettoChromeProducer(ctx context.Context, s *testing.State) {
	_, _, err := perfetto.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running: ", err)
	}

	if err = waitForChromeProducer(ctx); err != nil {
		s.Fatal("Failed in waiting for Chrome producers: ", err)
	}
}
