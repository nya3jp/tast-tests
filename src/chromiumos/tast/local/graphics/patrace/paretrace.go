// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package patrace provides a function to replay a PATrace (GLES)
// (https://github.com/ARM-software/patrace) in android
package patrace

import (
	"context"
	"encoding/json"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// setPerf reads the performance numbers from the result file of paretrace, and
// store the values in perfValues
func setPerf(ctx context.Context, a *arc.ARC, perfValues *perf.Values, resultPath string) error {
	buf, err := a.ReadFile(ctx, resultPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read result file %q; paretrace did not finish successfully", resultPath)
	}

	var m struct {
		Results []struct {
			Time float64 `json:"time"`
			FPS  float64 `json:"fps"`
		} `json:"result"`
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		return err
	}

	result := m.Results[0]

	perfValues.Set(
		perf.Metric{
			Name:      "duration",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}, result.Time)
	perfValues.Set(
		perf.Metric{
			Name:      "fps",
			Unit:      "fps",
			Direction: perf.BiggerIsBetter,
			Multiple:  false,
		}, result.FPS)

	testing.ContextLogf(ctx, "Duration: %fs, fps: %f", result.Time, result.FPS)

	return nil
}
