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
		Impl:            newProfilerFixture(),
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
	})
}

type mode string

const (
	modeStat       mode = "stat"
	modeSched      mode = "sched"
	modeStatRecord mode = "statrecord"
	modeRecord     mode = "record"
)

type profilerFixture struct {
	modes []mode
	// Store outDir to keep results from profiler in fixture specific dir
	// ie use /tast/results/..../fixtures/profilerRunning instead of /tast/results/..../tests
	// This is done to maintain consistency with keeping logs gathered by fixtures independent of those gathered by tests
	outDir       string
	runningProfs *RunningProf
}

// newProfilerFixture creates new profilerFixture struct
func newProfilerFixture() *profilerFixture {
	return &profilerFixture{}
}

// newProfilers creates an array of profilers from runtime var args, also sets f.modes to corresponding modes.
func (f *profilerFixture) newProfilers() ([]Profiler, error) {
	var profs []Profiler
	var stat PerfStatOutput
	var sched PerfSchedOutput

	args := strings.Split(profilerMode.Value(), ",")

	for _, arg := range args {
		switch mode(arg) {
		case modeStat:
			f.modes = append(f.modes, modeStat)
			profs = append(profs, Perf(PerfStatOpts(&stat, 0)))
		case modeSched:
			f.modes = append(f.modes, modeSched)
			profs = append(profs, Perf(PerfSchedOpts(&sched, "")))
		case modeRecord:
			f.modes = append(f.modes, modeRecord)
			profs = append(profs, Perf(PerfRecordOpts("", nil, PerfRecordCallgraph)))
		case modeStatRecord:
			f.modes = append(f.modes, modeStatRecord)
			profs = append(profs, Perf(PerfStatRecordOpts()))
		default:
			return nil, errors.Errorf("Unidentified profiler: %s not recognized, cannot start profiler", string(arg))
		}
	}
	return profs, nil
}

// filePaths gets paths to all files written by a profiler.
func (f *profilerFixture) filePaths(outDir string) []string {
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

// filesExist checks if the specified files have been created.
func filesExist(files []string) (bool, error) {
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
	// TODO(jacobraz): handle aarch64 devices that cant run perf
	f.outDir = s.OutDir()
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
	profs, err := f.newProfilers()
	if err != nil {
		s.Error("Failure in starting the profiler: ", err)
		return
	}
	if profs == nil {
		return
	}
	rp, err := Start(s.TestContext(), f.outDir, profs...)
	if err != nil {
		s.Error("Failure in starting the profiler: ", err)
		return
	}
	f.runningProfs = rp

	outFiles := f.filePaths(f.outDir)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if ok, err := filesExist(outFiles); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check for file"))
		} else if !ok {
			return errors.New("failed waiting for file(s)" + strings.Join(outFiles, " "))
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Error("Failed to wait for profiler file creation: ", err)
	}
}

func (f *profilerFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.runningProfs == nil {
		return
	}
	if err := f.runningProfs.End(s.TestContext()); err != nil {
		s.Error("Failure in ending the profiler: ", err)
	}
}
