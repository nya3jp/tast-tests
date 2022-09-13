// Copyright 2022 The ChromiumOS Authors
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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

const (
	dutPowerCmd         = "dut-power"
	dutPowerOutputDir   = "/tmp/dut-power-output"
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
	// Clear any files from a previous run
	if err := p.h.ServoProxy.RunCommand(p.ctx, false, "rm", "-rf", dutPowerOutputDir); err != nil {
		e := fmt.Sprintf("failed to clear tmp files on proxy: %s", err)
		return nil, errors.New(e)
	}

	// Run dut-power on the servod host to get our measurements
	time := strconv.Itoa(int(duration.Seconds()))
	err := p.h.ServoProxy.RunCommand(p.ctx, false, dutPowerCmd, "-o", dutPowerOutputDir, "-t", time, "--save-json")
	if err != nil {
		e := fmt.Sprintf("failed to run dut-power: %s", err)
		return nil, errors.New(e)
	}

	dutPowerLocalDir, err := os.MkdirTemp("", dutPowerTmpPrefix)
	if err != nil {
		e := fmt.Sprintf("failed to create temp results directory: %s", err)
		return nil, errors.New(e)
	}

	// Prioritize attempt to use the onboard summaries first then fall back to the EC
	// The accumulator summary may not be present in certain cases, e.g.
	// short measurement times
	logPaths := []string{
		onboardAccumSummary,
		onboardSummary,
		ecSummary,
	}

	var contents = []byte{}
	for i := 0; i < len(logPaths); i++ {
		remotePath := path.Join(dutPowerOutputDir, logPaths[i])
		localPath := path.Join(dutPowerLocalDir, logPaths[i])
		contents, err = p.readRemoteFile(remotePath, localPath)

		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, errors.New("failed to read measurement logs")
	}

	measurements := make(map[string]measurement)
	if err := json.Unmarshal(contents, &measurements); err != nil {
		e := fmt.Sprintf("failed to decode results: %s", err)
		return nil, errors.New(e)
	}

	means := make(map[string]float32)
	for key, value := range measurements {
		means[key] = value.Mean
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
