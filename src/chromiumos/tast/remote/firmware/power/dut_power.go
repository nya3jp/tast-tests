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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

const (
	dutPowerCmd       = "dut-power"
	dutPowerOutputDir = "/tmp/dut-power-output"
	dutPowerTmpPrefix = "dut-power-results"
	ecSummary         = "ec_summary.json"
	onboardSummary    = "onboard.accum_summary.json"
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

	// Try to use measurements from on-board sensors first
	onboardRemotePath := path.Join(dutPowerOutputDir, onboardSummary)
	onboardLocalPath := path.Join(dutPowerLocalDir, onboardSummary)
	contents, err := p.readRemoteFile(onboardRemotePath, onboardLocalPath)
	if err != nil {
		// See if we can fall back to any EC measurements
		oldErr := err
		ecRemotePath := path.Join(dutPowerOutputDir, ecSummary)
		ecLocalPath := path.Join(dutPowerLocalDir, ecSummary)
		contents, err = p.readRemoteFile(ecRemotePath, ecLocalPath)
		if err != nil {
			e := fmt.Sprintf("failed to read onboard and ec measurements: %s, %s", oldErr, err)
			return nil, errors.New(e)
		}
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
