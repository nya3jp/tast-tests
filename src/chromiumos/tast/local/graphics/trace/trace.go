// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides common code to run graphics trace files.
package trace

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	logDir  = "trace"
	envFile = "glxinfo.txt"
)

// Options has options for the trace replay.
type Options struct {
	HasFrameLoop   bool
	FrameLoopBegin int
	FrameLoopEnd   int
	FrameLoopCount int
}

// Option is a self-referential function that can be used to configure traces. See https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md
type Option func(c *Options)

// FrameLoop returns an option to specify frames to loop during replay.
func FrameLoop(begin int, end int, count int) Option {
	return func(options *Options) {
		options.HasFrameLoop = true
		options.FrameLoopBegin = begin
		options.FrameLoopEnd = end
		options.FrameLoopCount = count
	}
}

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
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()

	if err := setupReplay(ctx, s, cont); err != nil {
		s.Fatal("Failed to setup for replaying: ", err)
	}

	for traceFile, traceName := range traces {
		shortCtx, st := timing.Start(shortCtx, "trace:"+traceName)
		defer st.End()
		perfValues, err := runTrace(shortCtx, cont, s.DataPath(traceFile), traceName)
		if err != nil {
			s.Fatal("Failed running trace: ", err)
		}
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
	}
}

// RunExtendedTest starts a VM and runs a single trace with additional options for repeating or CPU loading.
func RunExtendedTest(ctx context.Context, s *testing.State, cont *vm.Container, traceFile string, traceName string, cpuThreads int, setters ...Option) {
	options := &Options{
		HasFrameLoop: false,
	}
	for _, setter := range setters {
		setter(options)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	if err := setupReplay(ctx, s, cont); err != nil {
		s.Fatal("Failed to setup for replaying: ", err)
	}

	ctx, st := timing.Start(ctx, "trace:"+traceName)
	defer st.End()

	containerPath, err := prepareTrace(ctx, cont, s.DataPath(traceFile))
	if err != nil {
		s.Fatal("Failed preparing trace: ", err)
	}
	defer cont.Command(ctx, "rm", "-f", containerPath).Run()

	// Synchronize shutdown.
	var wait sync.WaitGroup
	wait.Add(cpuThreads)

	// Create N threads.
	testing.ContextLogf(ctx, "Starting %d CPU threads", cpuThreads)
	for i := 0; i < cpuThreads; i++ {
		go func(t int) {
			testing.ContextLog(ctx, "Starting thread ", t)
			x := uint64(0)
			for {
				select {
				case <-ctx.Done():
					testing.ContextLogf(ctx, "Finished after %d iterations", x)
					wait.Done()
					return
				default:
					useCPU()
					x++
				}
			}
		}(i)
	}

	// Start replay.
	perfValues, err := replayTrace(ctx, cont, containerPath, traceName, options)
	if err != nil {
		s.Fatal("Failed running trace: ", err)
	}

	// Signal CPU threads to exit and signal completion.
	cancel()

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}

	// Wait for all threads to complete.
	wait.Wait()
}

// setupReplay prepares a container for replaying traces.
func setupReplay(ctx context.Context, s *testing.State, cont *vm.Container) error {
	outDir := filepath.Join(s.OutDir(), logDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create output dir %v", outDir)
	}
	file := filepath.Join(outDir, envFile)
	s.Log("Logging container graphics environment to ", envFile)
	if err := logInfo(ctx, cont, file); err != nil {
		s.Log("Failed to log container information: ", err)
	}

	s.Log("Checking if apitrace installed")
	cmd := cont.Command(ctx, "sudo", "dpkg", "-l", "apitrace")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to log container information")
	}

	return nil
}

// runTrace runs a trace and writes output to ${traceName}.txt. traceFile should be absolute path.
func runTrace(ctx context.Context, cont *vm.Container, traceFile, traceName string) (*perf.Values, error) {
	containerPath, err := prepareTrace(ctx, cont, traceFile)
	if err != nil {
		return nil, err
	}
	defer cont.Command(ctx, "rm", "-f", containerPath).Run()

	return replayTrace(ctx, cont, containerPath, traceName, nil)
}

// prepareTrace pushes a trace to the DUT and decompresses it prior to replay.
func prepareTrace(ctx context.Context, cont *vm.Container, traceFile string) (string, error) {
	containerPath := filepath.Join("/tmp", filepath.Base(traceFile))
	if err := cont.PushFile(ctx, traceFile, containerPath); err != nil {
		return "", errors.Wrap(err, "failed copying trace file to container")
	}

	containerPath, err := decompressTrace(ctx, cont, containerPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to decompress trace")
	}

	return containerPath, nil
}

// replayTrace replays a trace and parses the results.
func replayTrace(ctx context.Context, cont *vm.Container, containerPath string, traceName string, options *Options) (*perf.Values, error) {
	testing.ContextLog(ctx, "Replaying trace file ", filepath.Base(containerPath))
	args := []string{"apitrace", "replay", containerPath}
	if deadline, ok := ctx.Deadline(); ok {
		d := int(time.Until(deadline).Seconds())
		args = append(args, fmt.Sprintf("--timeout=%d", d))
		if options != nil && options.HasFrameLoop {
			args = append(args, fmt.Sprintf("--loop-begin=%d", options.FrameLoopBegin))
			args = append(args, fmt.Sprintf("--loop-end=%d", options.FrameLoopEnd))
			args = append(args, fmt.Sprintf("--loop-repeat-cnt=%d", options.FrameLoopCount))
		}
	}
	ctx, st := timing.Start(ctx, "replay")
	defer st.End()
	cmd := cont.Command(ctx, args...)
	traceOut, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		testing.ContextLog(ctx, string(traceOut))
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
	ctx, st := timing.Start(ctx, "decompress")
	defer st.End()

	var decompressFile string
	testing.ContextLog(ctx, "Decompressing trace file ", traceFile)
	ext := filepath.Ext(traceFile)
	switch ext {
	case ".trace":
		decompressFile = traceFile
	case ".bz2":
		if _, err := cont.Command(ctx, "bunzip2", traceFile).CombinedOutput(testexec.DumpLogOnError); err != nil {
			return "", errors.Wrap(err, "failed to decompress bz2")
		}
		decompressFile = strings.TrimSuffix(traceFile, filepath.Ext(traceFile))
	case ".zst", ".xz":
		if _, err := cont.Command(ctx, "zstd", "-d", "-f", "--rm", "-T0", traceFile).CombinedOutput(testexec.DumpLogOnError); err != nil {
			return "", errors.Wrap(err, "failed to decompress zst")
		}
		decompressFile = strings.TrimSuffix(traceFile, filepath.Ext(traceFile))
	default:
		return "", errors.Errorf("unknown trace extension: %s", ext)
	}
	return decompressFile, nil
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

func useCPU() int64 {
	// Do stuff here.
	cnt := int64(time.Now().Unix())
	for i := int64(0); i < 1000000; i++ {
		// TODO(davidriley): Add memory accesses.
		cnt += i
	}

	return cnt
}

// TODO(pwang): Write a func to cleans up disk in best effort.
