// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides common code to run graphics trace files.
package trace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	logDir  = "trace"
	envFile = "glxinfo.txt"
)

func logInfo(ctx context.Context, cont *vm.Container, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := cont.Command(ctx, "glxinfo")
	cmd.Stdout, cmd.Stderr = f, f
	return cmd.Run()
}

// RunTest starts a VM and runs all traces in trace, which maps from filenames (passed to s.DataPath) to a human-readable name for the trace, that is used both for the output file's name and for the reported perf keyval.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container, traces map[string]string) {
	outDir := filepath.Join(s.OutDir(), logDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		s.Fatalf("Failed to create output dir %v: %v", outDir, err)
	}
	file := filepath.Join(outDir, envFile)
	s.Log("Logging container graphics environment to ", envFile)
	if err := logInfo(ctx, cont, file); err != nil {
		s.Log("Failed to log container information: ", err)
	}

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()

	s.Log("Checking if apitrace installed")
	cmd := cont.Command(shortCtx, "sudo", "dpkg", "-l", "apitrace")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(shortCtx)
		s.Fatal("Failed to get apitrace: ", err)
	}
	for traceFile, traceName := range traces {
		perfValues, err := runTrace(shortCtx, cont, s.DataPath(traceFile), traceName)
		if err != nil {
			s.Fatal("Failed running trace: ", err)
		}
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
	}
}

// runTrace runs a trace and writes output to ${traceName}.txt. traceFile should be absolute path.
func runTrace(ctx context.Context, cont *vm.Container, traceFile, traceName string) (*perf.Values, error) {
	containerPath := filepath.Join("/tmp", filepath.Base(traceFile))
	if err := cont.PushFile(ctx, traceFile, containerPath); err != nil {
		return nil, errors.Wrap(err, "failed copying trace file to container")
	}

	containerPath, err := decompressTrace(ctx, cont, containerPath)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Replaying trace file ", filepath.Base(containerPath))
	cmd := cont.Command(ctx, "apitrace", "replay", containerPath)
	traceOut, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrap(err, "failed to replay apitrace")
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get OutDir for writing trace result")
	}
	// Suggesting the file is human readable by appending txt extension.
	file := filepath.Join(outDir, logDir, traceName+".txt")
	testing.ContextLog(ctx, "Dumping trace output to file ", filepath.Base(file))
	if err := ioutil.WriteFile(file, traceOut, 0644); err != nil {
		return nil, errors.Wrap(err, "error writing tracing output")
	}
	return parseResult(traceName, string(traceOut))
}

// decompressTrace trys to decompress the trace into trace format if possible. If the input is uncompressed, this function will do nothing.
// Returns the uncompressed file absolute path.
func decompressTrace(ctx context.Context, cont *vm.Container, traceFile string) (string, error) {
	if filepath.Ext(traceFile) != ".bz2" {
		return traceFile, nil
	}
	testing.ContextLog(ctx, "Decompressing trace file ", traceFile)
	cmd := cont.Command(ctx, "bunzip2", traceFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		testing.ContextLog(ctx, string(output))
		return "", errors.Wrap(err, "failed to decompress bz2")
	}
	return strings.TrimSuffix(traceFile, filepath.Ext(traceFile)), nil
}

// parseResult parses the output of apitrace and return the perfs.
func parseResult(traceName, output string) (*perf.Values, error) {
	re := regexp.MustCompile(`Rendered (\d+) frames in (\d*\.?\d*) secs, average of (\d*\.?\d*) fps`)
	match := re.FindStringSubmatch(output)
	if match == nil {
		return nil, errors.New("result line can't be located")
	}

	frames, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse frames %q", match[1])
	}
	duration, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse duration %q", match[2])
	}
	fps, err := strconv.ParseFloat(match[3], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse fps %q", match[3])
	}

	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      traceName,
		Variant:   "time",
		Unit:      "sec",
		Direction: perf.SmallerIsBetter,
	}, duration)
	value.Set(perf.Metric{
		Name:      traceName,
		Variant:   "frames",
		Unit:      "frame",
		Direction: perf.BiggerIsBetter,
	}, float64(frames))
	value.Set(perf.Metric{
		Name:      traceName,
		Variant:   "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, fps)
	return value, nil
}

// TODO(pwang): Write a func to cleans up disk in best effort.
