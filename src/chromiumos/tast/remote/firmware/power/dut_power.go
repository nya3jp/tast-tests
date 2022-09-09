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
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

const (
	dutPowerCmd       = "dut-power"
	dutPowerTmpPrefix = "dut-power-results"
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
	summaries, err := p.MeasureAllSummaries(duration)
	if err != nil {
		return nil, err
	}

	for _, name := range []string{
		"ec",
		"onboard.accum",
		"onboard",
	} {
		summary, exists := summaries[name]
		if !exists {
			continue
		}
		results := NewResultsGeneric()
		results.AddMeans(summary)
		return &results, nil
	}
	return nil, errors.New("failed to find any results")
}

// MeasureAllSummaries returns all summaries created by dut-power, not just the
// most accurate value provided by Measure above.
//
// Returns a map of summary name (e.g. "onboard" for onboard_summary.json) to
// a map of metrics name (e.g. "ppdut5") to mean value.
func (p *DutPowerContext) MeasureAllSummaries(duration time.Duration) (map[string]map[string]float32, error) {
	// Run dut-power on the servod host to get our measurements
	time := strconv.Itoa(int(duration.Seconds()))

	output, err := p.h.ServoProxy.OutputCommand(p.ctx, false, dutPowerCmd, "-t", time, "--save-json")
	if err != nil {
		e := fmt.Sprintf("failed to run dut-power: %s", err)
		return nil, errors.New(e)
	}

	dutPowerLocalDir, err := os.MkdirTemp("", dutPowerTmpPrefix)
	if err != nil {
		e := fmt.Sprintf("failed to create temp results directory: %s", err)
		return nil, errors.New(e)
	}

	result := make(map[string]map[string]float32)
	for _, line := range strings.Split(string(output), "\n") {
		const summaryFileSuffix = "_summary.json"
		if strings.HasSuffix(line, summaryFileSuffix) {
			remotePath := line
			file := path.Base(remotePath)
			localPath := path.Join(dutPowerLocalDir, file)
			contents, err := p.readRemoteFile(remotePath, localPath)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read dut-power summary file")
			}
			measurements := make(map[string]measurement)
			if err := json.Unmarshal(contents, &measurements); err != nil {
				return nil, errors.Wrap(err, "failed to decode summary")
			}
			summary := make(map[string]float32)
			for key, value := range measurements {
				summary[key] = value.Mean
			}
			name := strings.TrimSuffix(file, summaryFileSuffix)
			result[name] = summary
		}
	}
	return result, nil
}

func (p *DutPowerContext) readRemoteFile(remotePath, localPath string) ([]byte, error) {
	if err := p.h.ServoProxy.GetFile(p.ctx, false, remotePath, localPath); err != nil {
		return nil, err
	}

	return os.ReadFile(localPath)
}
