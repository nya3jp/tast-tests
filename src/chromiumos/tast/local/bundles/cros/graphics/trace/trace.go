// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides common code to run graphics trace files.
package trace

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	logDir  = "trace"
	envFile = "glxEnv.txt"
)

// Result holds the performance metrics reported by apitrace.
type Result struct {
	// The average FPS.
	FPS float64
	// The time spent in replaying the trace.
	Duration float64
	// The total frames renderered by the trace.
	Frames uint64
}

// LogInfo logs graphics information of the container.
func LogInfo(ctx context.Context, cont *vm.Container, file string) error {
	cmd := cont.Command(ctx, "glxinfo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "glxinfo failed")
	}
	if err := ioutil.WriteFile(file, out, 0644); err != nil {
		return errors.Wrapf(err, "error writing %s", file)
	}
	return nil
}

// RunTest start an VM and runs the trace from the traceNameMap, which traceNameMap mapping from local file name to trace name.
func RunTest(ctx context.Context, s *testing.State, traceNameMap map[string]string) {
	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.LiveComponent)
	err = vm.SetUpComponent(ctx, vm.LiveComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.LiveImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to dump container log: ", err)
		}
		vm.StopConcierge(ctx)
	}()

	outDir := filepath.Join(s.OutDir(), logDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		s.Fatal("Failed to create output dir %v: %v", outDir, err)
	}
	file := filepath.Join(outDir, envFile)
	s.Log("Log container gfx environment to %s", file)
	if err := LogInfo(ctx, cont, file); err != nil {
		s.Log("Failed to log container information: ", err)
	}

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()

	// TODO(pwang): Install it in container image.
	s.Log("Installing apitrace")
	cmd := cont.Command(shortCtx, "sudo", "apt", "-y", "install", "apitrace")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(shortCtx)
		s.Fatal("Failed to get Apitrace: ", err)
	}
	for traceFileName, traceName := range traceNameMap {
		outputFile := filepath.Join(outDir, traceName)
		result := runTrace(shortCtx, s, cont, traceFileName, outputFile)
		perfValue := result.GeneratePerfs(traceName)
		if err := perfValue.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
	}
}

func runTrace(ctx context.Context, s *testing.State, cont *vm.Container, traceFileName string, outputFile string) Result {
	s.Log("Copy trace file to container")
	containerPath := filepath.Join("/home/testuser", traceFileName)
	if err := cont.PushFile(ctx, s.DataPath(traceFileName), containerPath); err != nil {
		s.Fatal("Failed copying trace file to container: ", err)
	}

	s.Log("Replay trace file")
	cmd := cont.Command(ctx, "apitrace", "replay", "--verbose", containerPath)
	traceOut, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to replay apitrace: ", err)
	}

	s.Logf("Dump trace output to file: %s", outputFile)
	if err := ioutil.WriteFile(outputFile, traceOut, 0644); err != nil {
		s.Fatal("Error writing tracing output: ", err)
	}
	result, err := parseResult(traceOut)
	if err != nil {
		s.Fatal("Failed to parse the result: ", err)
	}
	return result
}

// GeneratePerfs generate the perf metrics for chromeperf upload.
func (result *Result) GeneratePerfs(testName string) *perf.Values {
	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      testName,
		Variant:   "time",
		Unit:      "sec",
		Direction: perf.SmallerIsBetter,
	}, result.Duration)
	value.Set(perf.Metric{
		Name:      testName,
		Variant:   "frames",
		Unit:      "frame",
		Direction: perf.BiggerIsBetter,
	}, float64(result.Frames))
	value.Set(perf.Metric{
		Name:      testName,
		Variant:   "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, result.FPS)
	return value
}

func parseResult(output []byte) (Result, error) {
	var result Result
	re := regexp.MustCompile(`Rendered (\d+) frames in (\d*\.?\d*) secs, average of (\d*\.?\d*) fps`)
	match := re.FindSubmatch(output)
	if match == nil {
		err := errors.New("result line can't be located")
		return result, err
	}
	var tmp = string(bytes.Join(match[1:], []byte{' '}))
	_, err := fmt.Sscanf(tmp, "%d %f %f", &result.Frames, &result.Duration, &result.FPS)
	if err != nil {
		err = errors.Wrapf(err, "tmp: %s", tmp)
		return result, err
	}
	return result, nil
}

// TODO(pwang): Write a func to cleans up disk in best effort.
