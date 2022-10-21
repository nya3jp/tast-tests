// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
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
	// TODO: get these from metrics_proto/execution_context.proto
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
		Func: StackSampledMetrics,
		// TODO: We need to add a similar 'GetSuccessfullyCollectedCounts' dbus
		// method to Lacros before we can add a Lacros test. The current dbus
		// service is ash-only.
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

	type ProcessThread struct {
		processType int
		threadType  int
	}
	// We always expect to see at least the following process + threads being profiled.
	// List should generally match chrome/common/profiler/thread_profiler_browsertest.cc
	expectedResults := []ProcessThread{
		{browserProcess, mainThread},
		{browserProcess, ioThread},
		{rendererProcess, mainThread},
		{rendererProcess, ioThread},
		{rendererProcess, compositorThread},
		{gpuProcess, mainThread},
		{gpuProcess, ioThread},
		{gpuProcess, compositorThread},
		{networkServiceProcess, ioThread},
	}

	testing.ContextLog(ctx, "Waiting for all processes + threads to be profiled")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		type result struct {
			ProcessType int
			ThreadType  int
			Count       int
		}
		var results []result
		if err := obj.CallWithContext(ctx, dbusName+"."+statusFunction, dbus.FlagNoAutoStart).Store(&results); err != nil {
			return errors.Wrap(err, "failed to get profiler status")
		}

		var missedExpections []ProcessThread
		for _, expectation := range expectedResults {
			found := false
			for _, result := range results {
				if expectation.processType == result.ProcessType && expectation.threadType == result.ThreadType && result.Count > 0 {
					found = true
					break
				}
			}

			if !found {
				missedExpections = append(missedExpections, expectation)
			}
		}

		if len(missedExpections) > 0 {
			var missedExpectionsStr []string
			for _, missedExpection := range missedExpections {
				missedExpectionsStr = append(missedExpectionsStr, fmt.Sprintf("%+v", missedExpection))
			}
			return errors.New("not all process + threads profiled: " + strings.Join(missedExpectionsStr, ", "))
		}

		return nil
	}, nil); err != nil {
		s.Error("Chrome did not profile expected process+threads: ", err)
	}
}
