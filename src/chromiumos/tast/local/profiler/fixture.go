// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package profiler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "profilerRunning",
		Desc:            "Started profilers specified by profiler.AccessVars.mode variable",
		Contacts:        []string{"jacobraz@google.com"},
		Impl:            NewProfilerFixture([]string{"stat", "sched", "record", "statrecord"}),
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
	})
}

const modeStat = "stat"
const modeSched = "sched"
const modeStatRecord = "statrecord"
const modeRecord = "record"

type profilerFixture struct {
	modes        []string
	outDir       string
	runningProfs *RunningProf
	perfCtx      context.Context
}

// NewProfilerFixture reates new profilerFixture struct with the specified mode
func NewProfilerFixture(mode []string) *profilerFixture {
	return &profilerFixture{modes: mode}
}

func (f *profilerFixture) GetProfs(s *testing.FixtTestState) []Profiler {
	var profs []Profiler
	var stat PerfStatOutput
	var sched PerfSchedOutput

	for _, arg := range f.modes {
		switch arg {
		case modeStat:
			profs = append(profs, Perf(PerfStatOpts(&stat, 0)))
		case modeSched:
			profs = append(profs, Perf(PerfSchedOpts(&sched, "")))
		case modeRecord:
			profs = append(profs, Perf(PerfRecordOpts()))
		case modeStatRecord:
			profs = append(profs, Perf(PerfStatRecordOpts()))
		case "none":
			return nil
		default:
			s.Errorf("Unidentified profiler: %v not recognized, cannot start profiler", arg)
		}
	}
	return profs
}

// getFilePaths gets paths to all files written by a profiler
func (f *profilerFixture) getFilePaths(ctx context.Context, outDir string) []string {
	var paths []string
	for _, arg := range f.modes {
		switch arg {
		case modeStat:
			paths = append(paths, filepath.Join(outDir, perfStatFileName))
		case modeSched:
			paths = append(paths, filepath.Join(outDir, perfSchedFileName))
		case modeRecord:
			paths = append(paths, filepath.Join(outDir, perfRecordFileName))
		case modeStatRecord:
			paths = append(paths, filepath.Join(outDir, perfStatRecordFileName))
		}
	}
	return paths
}

// MustSucceedEval checks if the specified files have been created
func MustSucceedEval(ctx context.Context, files []string) (bool, error) {
	success := true
	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			continue
		} else if errors.Is(err, os.ErrNotExist) {
			success = false
		} else {
			return false, err
		}
	}
	return success, nil
}

func (f *profilerFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	//TODO handle aarch64 devices that cant run perf
	f.outDir = s.OutDir()
	f.perfCtx = s.FixtContext()
	return nil
}

func (f *profilerFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// All necessary work for tearing down the profiler is done in PostTest
}

func (f *profilerFixture) Reset(ctx context.Context) error {
	// All necessary work for resetting the profiler state is done in pre/post test
	return nil
}

func (f *profilerFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	profs := f.GetProfs(s)
	rp, err := Start(f.perfCtx, f.outDir, profs...)
	if err != nil {
		s.Error("Failure in starting the profiler: ", err)
	}
	f.runningProfs = rp

	outFiles := f.getFilePaths(ctx, f.outDir)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if test, err := MustSucceedEval(ctx, outFiles); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check for file"))
		} else if test {
			return nil
		}
		return errors.New("failed waiting for file(s)" + strings.Join(outFiles, " "))
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Error("Failed to wait for profiler file creation: ", err)
	}
	return
}

func (f *profilerFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.runningProfs == nil {
		return
	}
	if err := f.runningProfs.End(f.perfCtx); err != nil {
		s.Error("Failure in ending the profiler: ", err)
	}
	return

}
