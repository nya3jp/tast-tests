// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"reflect"

	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

const (
	traceConfig = "perfetto_trace_cfg.pbtxt"
	traceQuery  = "perfetto_trace_query.sql"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerfettoTraceProcessing,
		Desc:     "Exercises post-processing perfetto traces",
		Contacts: []string{"chinglinyu@chromium.org", "baseos-perf@google.com"},
		Data: []string{traceConfig,
			traceQuery,
			tracing.TraceProcessorAmd64,
			tracing.TraceProcessorArm,
			tracing.TraceProcessorArm64},
		Attr: []string{"group:mainline", "informational"},
	})
}

// PerfettoTraceProcessing exercises tracing.Session.RunQuery() and
// tracing.Session.RunQueryString() functions to show how to post-process a
// collected trace using a SQL query.
func PerfettoTraceProcessing(ctx context.Context, s *testing.State) {
	// We don't need to run any test action during the tracing session.
	// Just use the blocking version of StartSession() for simplicity.
	sess, err := tracing.StartSessionAndWaitUntilDone(ctx, s.DataPath(traceConfig))
	// The temporary file of trace data is no longer needed when returned.
	defer sess.RemoveTraceResultFile()

	if err != nil {
		s.Fatal("Failed to start tracing: ", err)
	}

	// Process the trace data using inline string query for simple queries.
	res1, err := sess.RunQueryString(ctx, s.DataPath(tracing.TraceProcessor()),
		"select cmdline from process where pid=1")
	if err != nil {
		s.Fatal("Failed to process the trace data: ", err)
	}
	if len(res1) != 2 || res1[1][0] != "/sbin/init" {
		s.Fatalf("Unexpected query result: %q", res1)
	}

	// Process the trace data using external SQL query file. This is preferable if the query is complex.
	res2, err := sess.RunQuery(ctx, s.DataPath(tracing.TraceProcessor()),
		s.DataPath(traceQuery))
	if err != nil {
		s.Fatal("Failed to process the trace data: ", err)
	}
	// We should get identical results using the same query.
	if !reflect.DeepEqual(res1, res2) {
		s.Fatalf("Unexpected query result: %q", res2)
	}
}
