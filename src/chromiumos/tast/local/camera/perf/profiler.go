// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package perf provides helper function to collect performance data like
// CPU and GPU usage, power, perf events and top. The profilers available are:
//   - CPU: Collects CPU and power usage. Will output perf metrics in
//     ProfilerContext.Results.
//   - GPU: Collects GPU usage. Will output perf metrics in
//     ProfilerContext.Results.
//   - PerfRecord: Collects perf data from perf tools sampling. Will write the
//     sampled perf_record.data to ProfilerContext.OutDir.
//   - Top: Samples top output. Will write output to top.data under
//     ProfilerContext.OutDir.
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

type taskfunc func() (*perf.Values, error)

// task defines a profiler task.
type task struct {
	name    string
	results *perf.Values
	err     error
	run     taskfunc
}

// ProfilerType refers to a profiler supported by this package.
type ProfilerType string

// The name of the available profilers.
const (
	CPU        ProfilerType = "cpu"
	GPU                     = "gpu"
	PerfRecord              = "perf_record"
	Top                     = "top"
)

// ProfilerContext holds the settings and results of a set of profiler tasks.
type ProfilerContext struct {
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
// profiler tasks in parallel. |profilers| is used to specify the set of
// profilers to run. We run CPU, GPU and Top profilers by default if |profilers|
// is not given.
func Start(ctx context.Context, sDur, mDur time.Duration, outPrefix, outdir string, profilers []ProfilerType) (*ProfilerContext, error) {
	pctx := ProfilerContext{
		StabilizeDuration: sDur,
		MeasureDuration:   mDur,
		OutputPrefix:      outPrefix,
		OutDir:            outdir,
	}

	allTaskfunc := map[ProfilerType]taskfunc{
		CPU: func() (*perf.Values, error) {
			pv := perf.NewValues()
			if err := graphics.MeasureCPUUsageAndPower(ctx, pctx.StabilizeDuration, pctx.MeasureDuration, pv); err != nil {
				return nil, errors.Wrap(err, "failed to measure CPU and power usage")
			}
			return pv, nil
		},
		GPU: func() (*perf.Values, error) {
			if err := testing.Sleep(ctx, pctx.StabilizeDuration); err != nil {
				return nil, errors.Wrap(err, "failed to wait for stabilization")
			}
			pv := perf.NewValues()
			if err := graphics.MeasureGPUCounters(ctx, pctx.MeasureDuration, pv); err != nil {
				return nil, errors.Wrap(err, "failed to measure GPU usage")
			}
			return pv, nil
		},
		PerfRecord: func() (*perf.Values, error) {
			return runProfiler(ctx, &pctx, PerfRecord, profiler.Perf(profiler.PerfRecordOpts("", nil, profiler.PerfRecordCallgraph)))
		},
		Top: func() (*perf.Values, error) {
			return runProfiler(ctx, &pctx, Top, profiler.Top(nil))
		},
	}

	if profilers == nil {
		// Run CPU, GPU and Top profilers by default. PerfRecord is not enabled by default
		// because it can consume a lot of disk space.
		pctx.tasks = []*task{
			{name: string(CPU), run: allTaskfunc[CPU]},
			{name: string(GPU), run: allTaskfunc[GPU]},
			{name: string(Top), run: allTaskfunc[Top]},
		}
	} else {
		for _, p := range profilers {
			if _, exists := allTaskfunc[p]; !exists {
				return nil, errors.Errorf("unknown profiler: %v", p)
			}
			pctx.tasks = append(pctx.tasks, &task{name: string(p), run: allTaskfunc[p]})
		}
	}

	for _, t := range pctx.tasks {
		pctx.wg.Add(1)
		go func(tsk *task) {
			defer pctx.wg.Done()
			tsk.results, tsk.err = tsk.run()
		}(t)
	}

	return &pctx, nil
}

// Wait waits all the profiler tasks to finish and merge the output perf metrics
// from different tasks into ProfilerContext.Results.
func (pctx *ProfilerContext) Wait() error {
	pctx.wg.Wait()

	pctx.Results = perf.NewValues()

	for _, t := range pctx.tasks {
		if t.err != nil {
			return errors.Wrapf(t.err, "failed to collect %s data", t.name)
		}
		if t.results != nil {
			convertFromProtobuf(t.results.Proto().GetValues(), pctx.Results, pctx.OutputPrefix)
		}
	}
	return nil
}

func convertFromProtobuf(pbv []*perfpb.Value, outputPV *perf.Values, outputPrefix string) {
	for _, v := range pbv {
		outputPV.Set(perf.Metric{
			Name:      fmt.Sprintf("%s-%s", outputPrefix, v.GetName()),
			Variant:   v.GetVariant(),
			Unit:      v.GetUnit(),
			Direction: perf.Direction(v.GetDirection()),
			Multiple:  v.GetMultiple(),
		}, v.Value...)
	}
}

func runProfiler(ctx context.Context, pctx *ProfilerContext, name string, prof profiler.Profiler) (*perf.Values, error) {
	if err := testing.Sleep(ctx, pctx.StabilizeDuration); err != nil {
		return nil, errors.Wrap(err, "failed to wait for stabilization")
	}
	rp, err := profiler.Start(ctx, pctx.OutDir, prof)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start %s profiler", name)
	}
	if err := testing.Sleep(ctx, pctx.MeasureDuration); err != nil {
		return nil, errors.Wrap(err, "failed to measure the full duration")
	}
	if err := rp.End(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to stop %s profiler", name)
	}
	return nil, nil
}
