// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	// TODO get these from metrics_proto/execution_context.proto
	browserProcess        = 1
	rendererProcess       = 2
	gpuProcess            = 3
	utilityProcess        = 4
	networkServiceProcess = 10

	mainThread       = 1
	ioThread         = 6
	compositorThread = 11
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StackSampledMetrics,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check that stack-sampled metrics work",
		Contacts: []string{
			"iby@chromium.org",
			"cros-telemetry@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "stack_sampled_metrics"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: "chromeLoggedInWithStackSampledMetrics",
			Val:     browser.TypeAsh,
		}},
	})
}

func StackSampledMetrics(ctx context.Context, s *testing.State) {
	const (
		dbusName       = "org.chromium.ProfilerStatusService"
		dbusPath       = "/org/chromium/ProfilerStatusService"
		statusFunction = "GetSuccessfullyCollectedCounts"
	)

	// Reserve a few seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	// Set up the browser, open a window.
	const url = chrome.NewTabURL
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), url)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)
	defer conn.Close()

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}
	testing.ContextLog(ctx, "Waiting for all processes + threads to be profiled")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		type result struct {
			Process int
			Thread  int
			Count   int
		}
		var results []result
		if err := obj.CallWithContext(ctx, dbusName+"."+statusFunction, dbus.FlagNoAutoStart).Store(&results); err != nil {
			return errors.Wrap(err, "failed to get profiler status")
		}

		// We always expect to see at least the following process + threads being profiled.
		// List should generally match chrome/common/profiler/thread_profiler_browsertest.cc
		expectedResults := []*result{
			&result{browserProcess, mainThread, 0},
			&result{browserProcess, ioThread, 0},
			&result{rendererProcess, mainThread, 0},
			&result{rendererProcess, ioThread, 0},
			&result{rendererProcess, compositorThread, 0},
			&result{gpuProcess, mainThread, 0},
			&result{gpuProcess, ioThread, 0},
			&result{gpuProcess, compositorThread, 0},
			&result{networkServiceProcess, ioThread, 0},
		}
		found := 0

		for _, result := range results {
			for _, expectation := range expectedResults {
				if expectation.Process == result.Process && expectation.Thread == result.Thread {
					if result.Count == 0 {
						continue
					}
					if expectation.Count != 0 {
						return testing.PollBreak(errors.Errorf("bad result from %s.%s: duplicate counts for process %d thread %d", dbusName, statusFunction, result.Process, result.Thread))
					}
					expectation.Count = result.Count
					found++
				}
			}
		}

		if found < len(expectedResults) {
			errString := "not all process + threads profiled: "
			for _, expectation := range expectedResults {
				if expectation.Count == 0 {
					errString += fmt.Sprintf("(%d, %d) ", expectation.Process, expectation.Thread)
				}
			}
			return errors.New(errString)
		}

		return nil
	}, nil); err != nil {
		s.Error("Chrome did not profile expected process+threads: ", err)
	}
}
