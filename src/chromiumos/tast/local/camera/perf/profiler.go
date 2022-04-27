// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf provides helper function to collect performance data like
// CPU and GPU usage, power, perf events and top.
package perf

import (
	"context"
	"fmt"
	"sync"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/perf/perfpb"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/profiler"
	"chromiumos/tast/testing"
)

// task defines a profiler task.
type task struct {
	name    string
	results *perf.Values
	err     error
	run     func() (*perf.Values, error)
}

// ProfilerContext holds the settings and results of a set of profiler tasks.
type ProfilerContext struct {
	// Ctx is the context.Context used by all the profiler tasks.
	Ctx context.Context

	// StabilizeDuration is the duration the profiler needs to wait for the
	// system to stabilize before start collecting profile data.
	StabilizeDuration time.Duration

	// MeasureDuration is the duration for which the profile data will be
	// collected.
	MeasureDuration time.Duration

	// OutputPrefix is a prefix string that will be prepended to the metric
	// names in Results.
	OutputPrefix string

	// OutDir is the directory that the profiler tasks will write their
	// output data to.
	OutDir string

	// Results is the output perf metrics collected by the profiler tasks.
	// Set when ProfilerContext.Wait() is called.
	Results *perf.Values

	// wg is the WaitGroup for all the launched profiler task goroutines.
	wg sync.WaitGroup

	// tasks is the list of profiler tasks to run.
	tasks []*task
}

// Start creates a ProfilerContext using the input arguments and start a set of
// profiler tasks in parallel. The profiler tasks will collect:
//   - CPU and power usage. Will output perf metrics in ProfilerContext.Results.
//   - GPU usage. Will output perf metrics in ProfilerContext.Results.
//   - perf data from perf tools sampling. Will write the sampled
//     perf_record.data to ProfilerContext.OutDir.
//   - top output sampling. Will write output to top.data under
//     ProfilerContext.OutDir.
func Start(ctx context.Context, sDur, mDur time.Duration, outPrefix, outdir string) *ProfilerContext {
	pctx := ProfilerContext{
		Ctx:               ctx,
		StabilizeDuration: sDur,
		MeasureDuration:   mDur,
		OutputPrefix:      outPrefix,
		OutDir:            outdir,
	}
	pctx.tasks = []*task{
		{name: "CPU_and_power", run: func() (*perf.Values, error) {
			pv := perf.NewValues()
			if err := graphics.MeasureCPUUsageAndPower(pctx.Ctx, pctx.StabilizeDuration, pctx.MeasureDuration, pv); err != nil {
				return nil, errors.Wrap(err, "failed to measure CPU and power usage")
			}
			return pv, nil
		}},
		{name: "GPU", run: func() (*perf.Values, error) {
			pv := perf.NewValues()
			if err := graphics.MeasureGPUCounters(pctx.Ctx, pctx.MeasureDuration, pv); err != nil {
				return nil, errors.Wrap(err, "failed to measure GPU usage")
			}
			return pv, nil
		}},
		{name: "PerfRecord", run: func() (*perf.Values, error) {
			perf := profiler.Perf(profiler.PerfRecordOpts())
			testing.Sleep(pctx.Ctx, pctx.StabilizeDuration)
			rp, err := profiler.Start(pctx.Ctx, pctx.OutDir, perf)
			if err != nil {
				return nil, errors.Wrap(err, "failed to start perf profiler")
			}
			testing.Sleep(pctx.Ctx, pctx.MeasureDuration)
			if err := rp.End(pctx.Ctx); err != nil {
				return nil, errors.Wrap(err, "failed to stop perf profiler")
			}
			return nil, nil
		}},
		{name: "Top", run: func() (*perf.Values, error) {
			top := profiler.Top(nil)
			testing.Sleep(pctx.Ctx, pctx.StabilizeDuration)
			rp, err := profiler.Start(pctx.Ctx, pctx.OutDir, top)
			if err != nil {
				return nil, errors.Wrap(err, "failed to start top profiler")
			}
			testing.Sleep(pctx.Ctx, pctx.MeasureDuration)
			if err := rp.End(pctx.Ctx); err != nil {
				return nil, errors.Wrap(err, "failed to stop top profiler")
			}
			return nil, nil
		}},
	}

	for _, t := range pctx.tasks {
		pctx.wg.Add(1)
		go func(tsk *task) {
			defer pctx.wg.Done()
			tsk.results, tsk.err = tsk.run()
		}(t)
	}

	return &pctx
}

// Wait waits all the profiler tasks to finish and merge the output perf metrics
// from different tasks into ProfilerContext.Results.
func (pctx *ProfilerContext) Wait() error {
	pctx.wg.Wait()

	pctx.Results = perf.NewValues()

	convertFromProtobuf := func(pbv []*perfpb.Value) {
		for _, v := range pbv {
			pctx.Results.Set(perf.Metric{
				Name:      fmt.Sprintf("%s-%s", pctx.OutputPrefix, v.GetName()),
				Variant:   v.GetVariant(),
				Unit:      v.GetUnit(),
				Direction: perf.Direction(v.GetDirection()),
				Multiple:  v.GetMultiple(),
			}, v.Value...)
		}
	}

	for _, t := range pctx.tasks {
		if t.err != nil {
			return errors.Wrapf(t.err, "failed to collect %s data", t.name)
		}
		if t.results != nil {
			convertFromProtobuf(t.results.Proto().GetValues())
		}
	}
	return nil
}
