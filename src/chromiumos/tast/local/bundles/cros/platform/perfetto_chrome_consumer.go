// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/platform/perfetto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoChromeConsumer,
		Desc:         "Tests Chrome DevTools protocol for collecting a system-wide trace via the system tracing service",
		Contacts:     []string{"chinglinyu@chromium.org", "chenghaoyang@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Data:         []string{perfetto.TraceConfigFile},
		Attr:         []string{"group:mainline", "informational"}, // TODO(crbug/1194540) remove "informational" after the test is stable.
	})
}

// PerfettoChromeConsumer tests Chrome as a perfetto trace consumer.
// The test enables the "EnablePerfettoSystemTracing" feature flag for Chrome and then collects a system-wide trace using the system backend connected to traced, the system tracing service daemon.
func PerfettoChromeConsumer(ctx context.Context, s *testing.State) {
	const (
		traceDataFile           = "trace.data.gz"
		traceCollectionDuration = time.Second
	)

	cleanupCtx := ctx

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	_, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Create the binary protobuf TraceConfig: unmarshal from pbtxt and then marshal to binary protobuf.
	traceConfigPath := s.DataPath(perfetto.TraceConfigFile)
	configTxt, err := ioutil.ReadFile(traceConfigPath)
	if err != nil {
		s.Fatal("Failed to read the trace config: ", err)
	}
	// Unmarshall the pbtxt and then marshall to binary protobuf.
	config := &trace.TraceConfig{}
	if err := proto.UnmarshalText(string(configTxt), config); err != nil {
		s.Fatal("Failed to unmarshal perfetto config: ", err)
	}
	configPb, err := proto.Marshal(config)
	if err != nil {
		s.Fatal("Failed to marshal perfetto config: err")
	}

	// triedToStopTracing means that cr.StopTracing(cleanupCtx) was already done, with or without success (if it failed then we have no reason to try again with the same timeout.)
	triedToStopTracing := false
	defer func() {
		if triedToStopTracing {
			return
		}
		if _, err := cr.StopTracing(cleanupCtx); err != nil {
			s.Error("Failed to stop tracing in cleanup phase: ", err)
		}
	}()

	if err := cr.StartSystemTracing(ctx, configPb); err != nil {
		s.Fatal("Failed to start tracing: ", err)
	}

	// The trace config contains a longer trace collection duration, but we explicitly stop tracing before the trace duration elapses.
	if err := testing.Sleep(ctx, traceCollectionDuration); err != nil {
		s.Fatalf("Failed to wait %v: %v", traceCollectionDuration, err)
	}

	// Set triedToStopTracing to true so we don't redo in the deferred cleanup phase.
	triedToStopTracing = true
	tr, err := cr.StopTracing(cleanupCtx)
	if err != nil {
		s.Fatal("Failed to stop tracing: ", err)
	}
	if tr == nil || len(tr.Packet) == 0 {
		s.Fatal("No trace data is collected")
	}
	// TODO(crbug/1194540): in addition to checking the number of trace packets, post-process the collected trace using trace_processor_shell to verify the trace data.
	if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(s.OutDir(), traceDataFile)); err != nil {
		s.Fatal("Failed to save trace to file: ", err)
	}
}
