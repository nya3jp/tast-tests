// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package croshealthd provides methods for running and obtaining output from
// cros_healthd commands.
package croshealthd

import (
	"context"
	"encoding/csv"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
)

// TelemCategory represents a category flag that can be passed to the
// cros_healthd telem command.
type TelemCategory string

// Categories for the cros_healthd telem command.
const (
	TelemCategoryBacklight         TelemCategory = "backlight"
	TelemCategoryBattery           TelemCategory = "battery"
	TelemCategoryBluetooth         TelemCategory = "bluetooth"
	TelemCategoryCachedVPD         TelemCategory = "cached_vpd"
	TelemCategoryCPU               TelemCategory = "cpu"
	TelemCategoryFan               TelemCategory = "fan"
	TelemCategoryMemory            TelemCategory = "memory"
	TelemCategoryStatefulPartition TelemCategory = "stateful_partition"
	TelemCategoryStorage           TelemCategory = "storage"
	TelemCategoryTimezone          TelemCategory = "timezone"
)

// FetchTelemetry runs cros_healthd's telem command with the given category and
// reads the CSV output into a two-dimensional array. It also dumps the output
// of the telem command to a file for debugging. An error is returned if there
// is a failure to obtain or parse telemetry info or if a line of output has an
// unexpected number of fields.
func FetchTelemetry(ctx context.Context, category TelemCategory, outDir string) ([][]string, error) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return nil, errors.Wrap(err, "failed to start cros_healthd")
	}

	b, err := testexec.CommandContext(ctx, "telem", "--category="+string(category)).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "command failed")
	}

	// Log output to file for debugging.
	path := filepath.Join(outDir, "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		return nil, errors.Wrapf(err, "failed to write output to %s", path)
	}

	records, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse output")
	}

	return records, nil
}
