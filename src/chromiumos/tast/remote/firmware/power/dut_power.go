// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to measure power through dut-power

package power

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

const (
	dutPowerCmd         = "dut-power"
	dutPowerTmpPrefix   = "dut-power-results"
	ecSummary           = "ec_summary.json"
	onboardAccumSummary = "onboard.accum_summary.json"
	onboardSummary      = "onboard_summary.json"
)

// DutPowerContext manages power measurements taken through dut-power
type DutPowerContext struct {
	h   *firmware.Helper
	ctx context.Context
}

type measurement struct {
	Mean float32
}

// NewDutPowerContext creates a DutPowerContext
func NewDutPowerContext(ctx context.Context, h *firmware.Helper) *DutPowerContext {
	return &DutPowerContext{
		h:   h,
		ctx: ctx,
	}
}

// Measure measures power consumption through dut-power
func (p *DutPowerContext) Measure(duration time.Duration) (Results, error) {
	// Run dut-power on the servod host to get our measurements
	time := strconv.Itoa(int(duration.Seconds()))
	output, err := p.h.ServoProxy.OutputCommand(p.ctx, false, dutPowerCmd, "-t", time, "--save-json")
	if err != nil {
		e := fmt.Sprintf("failed to run dut-power: %s", err)
		return nil, errors.New(e)
	}

	// Build a map of output files to their paths.
	outputFiles := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "/power_measurements/") {
			outputFiles[path.Base(line)] = line
		}
	}

	dutPowerLocalDir, err := os.MkdirTemp("", dutPowerTmpPrefix)
	if err != nil {
		e := fmt.Sprintf("failed to create temp results directory: %s", err)
		return nil, errors.New(e)
	}

	// Read all results from the following files, if they exist.
	means := make(map[string]float32)
	for _, logPath := range []string{
		onboardAccumSummary,
		onboardSummary,
		ecSummary,
	} {
		remotePath, inLog := outputFiles[logPath]
		if !inLog {
			continue
		}
		localPath := path.Join(dutPowerLocalDir, logPath)
		contents, err := p.readRemoteFile(remotePath, localPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read dut-power result file")
		}

		measurements := make(map[string]measurement)
		if err := json.Unmarshal(contents, &measurements); err != nil {
			return nil, errors.Wrap(err, "failed to decode results")
		}
		for key, value := range measurements {
			means[key] = value.Mean
		}
	}

	results := NewResultsGeneric()
	results.AddMeans(means)
	return &results, nil
}

func (p *DutPowerContext) readRemoteFile(remotePath, localPath string) ([]byte, error) {
	if err := p.h.ServoProxy.GetFile(p.ctx, false, remotePath, localPath); err != nil {
		return nil, err
	}

	return os.ReadFile(localPath)
}
