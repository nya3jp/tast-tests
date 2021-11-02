// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package benchmark provides utilities for running Google Benchmark binaries on
// device.
package benchmark

import (
	"context"
	"encoding/json"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// Context is a struct representing the device context as probed by the
// benchmark binary.
type Context struct {
	Date              string `json:"date"`
	HostName          string `json:"host_name"`
	Executable        string `json:"executable"`
	NumCPUs           int    `json:"num_cpus"`
	MhzPerCPU         int    `json:"mhz_per_cpu"`
	CPUScalingEnabled bool   `json:"cpu_scaling_enabled"`
	Cached            []struct {
		Type       string `json:"type"`
		Level      int    `json:"level"`
		Size       int    `json:"size"`
		NumSharing int    `json:"num_sharing"`
	} `json:"caches"`
	LoadAvg          []float32 `json:"load_avg"`
	LibraryBuildType string    `json:"library_build_type"`
}

// Output is a struct representing one benchmark execution output.
type Output struct {
	Name                   string  `json:"name"`
	FamilyIndex            int     `json:"family_index"`
	PerFamilyInstanceIndex int     `json:"per_family_instance_index"`
	RunName                string  `json:"run_name"`
	RunType                string  `json:"run_type"`
	Repetitions            int     `json:"repetitions"`
	RepetitionIndex        int     `json:"repetition_index"`
	Threads                int     `json:"threads"`
	Iterations             int     `json:"iterations"`
	RealTime               float64 `json:"real_time"`
	CPUTime                float64 `json:"cpu_time"`
	TimeUnit               string  `json:"time_unit"`
}

// Result is a struct representing the complete result of one benchmark run.
type Result struct {
	Context    Context  `json:"context"`
	Benchmarks []Output `json:"benchmarks"`
}

// Format defines the output formatting of the benchmark run.
type Format int

const (
	// Console sets the output formatting to human-readable texts. This is
	// the default used by Google Benchmark.
	Console Format = iota

	// JSON sets the output formatting to JSON.
	JSON

	// CSV sets the output format to CSV.
	CSV
)

// Benchmark encapsulates all the context for running a benchmark binary.
type Benchmark struct {
	// executable is the path to the benchmark executable binary.
	executable string

	// filter is a string pattern as defined by the Google Benchmark for
	// specifying the sub-benchmark(s) to run.
	filter string

	// outFile is an optional file path for storing another copy of the
	// benchmark result in addition to the one written to stdout.
	outFile string

	// outResultFormat specifies the output formatting of the benchmark
	// result writted to outFile.
	outResultFormat Format

	// extraArgs is a list of strings specifying the extra arguments that
	// will be fed to the benchmark binary.
	extraArgs []string
}

// options is a self-referencing closure for configuring Benchmark.
type option func(b *Benchmark)

// Filter sets the benchmark name filter pattern.
func Filter(pattern string) option {
	return func(b *Benchmark) { b.filter = pattern }
}

// OutputFile sets the additional output file path.
func OutputFile(file string) option {
	return func(b *Benchmark) { b.outFile = file }
}

// OutputResultFormat sets the output formatting for the additional output file.
func OutputResultFormat(format Format) option {
	return func(b *Benchmark) { b.outResultFormat = format }
}

// ExtraArgs sets a list of arguments that will be passed to the benchmark
// binary.
func ExtraArgs(args ...string) option {
	return func(b *Benchmark) { b.extraArgs = args }
}

// New creates a Benchmark instance with the given options.
func New(exec string, opts ...option) *Benchmark {
	ret := &Benchmark{
		executable: exec,
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// Args returns an array of string for benchmark execution.
func (b *Benchmark) Args() []string {
	args := []string{b.executable}

	// Always set the output formatting to JSON for stdout, so that we can
	// parse and return the JSON-based Result instance.
	args = append(args, "--benchmark_format=json")

	if b.filter != "" {
		args = append(args, "--benchmark_filter="+b.filter)
	}

	if b.outFile != "" {
		args = append(args, "--benchmark_out="+b.outFile)
		switch b.outResultFormat {
		case Console:
			args = append(args, "--benchmark_out_format=console")
		case JSON:
			args = append(args, "--benchmark_out_format=json")
		case CSV:
			args = append(args, "--benchmark_out_format=csv")
		}
	}

	if len(b.extraArgs) > 0 {
		args = append(args, b.extraArgs...)
	}
	return args
}

// Run executes the benchmark and returns the benchmark result in a byte array.
func (b *Benchmark) Run(ctx context.Context) (*Result, error) {
	args := b.Args()
	cmd := testexec.CommandContext(ctx, args[0], args[1:]...)
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to run the benchmark binary: %s", b.executable)
	}

	var ret Result
	if err := json.Unmarshal(output, &ret); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal output bytes to JSON-based Result")
	}
	return &ret, nil
}
