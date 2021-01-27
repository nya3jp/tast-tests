// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides utilities related to the trace data of Chrome.
package trace

import (
	"compress/gzip"
	"context"
	"os"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CleanupDuration is the duration which will be secured to cleanup the tracing
// on Run.
const CleanupDuration = 2 * time.Second

// Traceable is the interface to collect trace data.
type Traceable interface {
	// StartTracing starts trace events collection for the selected categories. Android
	// categories must be prefixed with "disabled-by-default-android ", e.g. for the
	// gfx category, use "disabled-by-default-android gfx", including the space.
	// Note: StopTracing should be called even if StartTracing returns an error.
	// Sometimes, the request to start tracing reaches the browser process, but there
	// is a timeout while waiting for the reply.
	StartTracing(ctx context.Context, categories []string) error

	// StopTracing stops trace collection and returns the collected trace events.
	StopTracing(ctx context.Context) (*trace.Trace, error)
}

// Run runs the given function f with tracing with the given categories, and
// store the trace data into the given filename when it passes.
func Run(ctx context.Context, tr Traceable, categories []string, filename string, f func(ctx context.Context) error) error {
	shortCtx, cancel := ctxutil.Shorten(ctx, CleanupDuration)
	defer cancel()
	tracingStopped := false
	defer func() {
		if tracingStopped {
			return
		}
		if _, stopErr := tr.StopTracing(ctx); stopErr != nil {
			testing.ContextLog(ctx, "Failed to stop tracing: ", stopErr)
		}
	}()
	if err := tr.StartTracing(shortCtx, categories); err != nil {
		// StartTracing may fail with timeout but tracing is actually started. Calling
		// Stop to ensure this is not the case.
		return errors.Wrap(err, "failed to start tracing")
	}
	if err := f(shortCtx); err != nil {
		return errors.Wrap(err, "failed to run the passed function")
	}
	traces, err := tr.StopTracing(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to stop tracing")
	}
	tracingStopped = true
	if len(traces.Packet) == 0 {
		testing.ContextLog(ctx, "No trace data is collected")
	}
	return SaveTraceToFile(ctx, traces, filename)
}

// SaveTraceToFile marshals the given trace into a binary protobuf format and
// saves it to a gzipped archive at the specified path.
func SaveTraceToFile(ctx context.Context, trace *trace.Trace, path string) error {
	data, err := proto.Marshal(trace)
	if err != nil {
		return errors.Wrap(err, "could not marshal trace to binary")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrap(err, "could not open file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close file: ", err)
		}
	}()

	writer := gzip.NewWriter(file)
	defer func() {
		if err := writer.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close gzip writer: ", err)
		}
	}()

	if _, err := writer.Write(data); err != nil {
		return errors.Wrap(err, "could not write the data")
	}

	if err := writer.Flush(); err != nil {
		return errors.Wrap(err, "could not flush the gzip writer")
	}

	return nil
}
