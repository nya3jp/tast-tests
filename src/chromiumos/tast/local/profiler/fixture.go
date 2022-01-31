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
		Impl:            NewProfilerFixture("record stat sched statrecord"),
		SetUpTimeout:    100 * time.Second,
		ResetTimeout:    5 * time.Second,
		TearDownTimeout: 100 * time.Second,
	})
}

type profilerFixture struct {
	modes        string
	outDir       string
	runningProfs *RunningProf
	perfCtx      context.Context
}

// NewProfilerFixture reates new profilerFixture struct with the specified mode
func NewProfilerFixture(mode string) *profilerFixture {
	return &profilerFixture{modes: mode}
}

func (f *profilerFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	//TODO handle aarch64 devices that cant run perf
	args := strings.Fields(f.modes)
	profs := make([]Profiler, 0)
	var stat PerfStatOutput
	var sched PerfSchedOutput

	for _, arg := range args {
		switch arg {
		case "stat":
			profs = append(profs, Perf(PerfStatOpts(&stat, 0)))
		case "sched":
			profs = append(profs, Perf(PerfSchedOpts(&sched, "")))
		case "record":
			profs = append(profs, Perf(PerfRecordOpts()))
		case "statrecord":
			profs = append(profs, Perf(PerfStatRecordOpts()))
		case "none":
			return nil
		default:
			s.Error("Unidentified profiler: " + arg + " not recognized, cannot start profiler.")
		}
	}
	f.outDir = s.OutDir()
	f.perfCtx = s.FixtContext()
	rp, err := Start(f.perfCtx, f.outDir, profs...)
	if err != nil {
		s.Error("Failure in starting the profiler: ", err)
	}
	f.runningProfs = rp

	outFiles := f.getFilePaths(f.perfCtx, f.outDir)
	if err := testing.Poll(f.perfCtx, func(ctx context.Context) error {
		if test, err := MustSucceedEval(f.perfCtx, outFiles); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check for file"))
		} else if test {
			return nil
		}
		return errors.New("failed waiting for file(s)" + strings.Join(outFiles, " "))
	}, &testing.PollOptions{Timeout: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for profiler file creation")
	}
	return nil
}

// getFilePaths gets paths to all files written by a profiler
func (f *profilerFixture) getFilePaths(ctx context.Context, outDir string) []string {
	paths := make([]string, 0)
	for _, arg := range strings.Fields(f.modes) {
		switch arg {
		case "stat":
			paths = append(paths, filepath.Join(outDir, perfStatFileName))
		case "sched":
			paths = append(paths, filepath.Join(outDir, perfSchedFileName))
		case "record":
			paths = append(paths, filepath.Join(outDir, perfRecordFileName))
		case "statrecord":
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

func (f *profilerFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.runningProfs == nil {
		return
	}
	if err := f.runningProfs.End(f.perfCtx); err != nil {
		s.Error("Failure in ending the profiler: ", err)
	}
	return
}

func (f *profilerFixture) Reset(ctx context.Context) error {
	if f.runningProfs == nil {
		return errors.New("no profiler running, cannot reset")
	}
	if err := f.runningProfs.End(f.perfCtx); err != nil {
		return errors.Wrap(err, "failure in ending the profiler")
	}
	args := strings.Fields(f.modes)
	profs := make([]Profiler, 0)

	var stat PerfStatOutput
	var sched PerfSchedOutput

	for _, arg := range args {
		switch arg {
		case "stat":
			profs = append(profs, Perf(PerfStatOpts(&stat, 0)))
		case "sched":
			profs = append(profs, Perf(PerfSchedOpts(&sched, "")))
		case "record":
			profs = append(profs, Perf(PerfRecordOpts()))
		case "statrecord":
			profs = append(profs, Perf(PerfStatRecordOpts()))
		case "none":
			return nil
		default:
			return errors.New("Unidentified profiler: " + arg + " not recognized, cannot start profiler.")

		}
	}
	rp, err := Start(f.perfCtx, f.outDir, profs...)
	if err != nil {
		return errors.Wrap(err, "failure in starting the profiler")
	}

	f.runningProfs = rp
	return nil
}

func (f *profilerFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *profilerFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
