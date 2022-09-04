// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package croshealthd provides methods for running and obtaining output from
// cros_healthd commands.
package croshealthd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
)

// TelemCategory represents a category flag that can be passed to the
// cros_healthd telem command.
type TelemCategory string

// TelemParams provides arguments for running `cros-health-tool telem`.
type TelemParams struct {
	// The category to pass as the --category flag.
	Category TelemCategory
	// Whether to check all pids for --process flag.
	AllPIDs bool
	// The pids to check as the --process flag.
	PIDs []int
}

// Categories for the cros_healthd telem command.
const (
	TelemCategoryAudio             TelemCategory = "audio"
	TelemCategoryBacklight         TelemCategory = "backlight"
	TelemCategoryBattery           TelemCategory = "battery"
	TelemCategoryBluetooth         TelemCategory = "bluetooth"
	TelemCategoryBootPerformance   TelemCategory = "boot_performance"
	TelemCategoryBus               TelemCategory = "bus"
	TelemCategoryCPU               TelemCategory = "cpu"
	TelemCategoryDisplay           TelemCategory = "display"
	TelemCategoryFan               TelemCategory = "fan"
	TelemCategoryGraphics          TelemCategory = "graphics"
	TelemCategoryInput             TelemCategory = "input"
	TelemCategoryMemory            TelemCategory = "memory"
	TelemCategoryNetwork           TelemCategory = "network"
	TelemCategorySensor            TelemCategory = "sensor"
	TelemCategoryStatefulPartition TelemCategory = "stateful_partition"
	TelemCategoryStorage           TelemCategory = "storage"
	TelemCategorySystem            TelemCategory = "system"
	TelemCategoryTimezone          TelemCategory = "timezone"
	TelemCategoryAudioHardware     TelemCategory = "audio_hardware"
)

// NotApplicable is the value printed for optional fields when they aren't
// populated.
const NotApplicable = "N/A"

// RunTelem runs cros-health-tool's telem command with the given params and
// returns the output. It also dumps the output to a file for debugging. An
// error is returned if there is a failure to run the command or save the output
// to a file.
func RunTelem(ctx context.Context, params TelemParams, outDir string) ([]byte, error) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return nil, errors.Wrap(err, "failed to start cros_healthd")
	}

	args := []string{"telem"}
	if params.Category != "" {
		args = append(args, fmt.Sprintf("--category=%s", params.Category))
	}
	if params.AllPIDs && len(params.PIDs) > 0 {
		return nil, errors.New("Select either all PIDs or a set of PIDs, but not both")
	}
	if params.AllPIDs {
		args = append(args, "--process=all")
	}
	if len(params.PIDs) > 0 {
		var pidStrs []string
		for _, pid := range params.PIDs {
			pidStrs = append(pidStrs, strconv.Itoa(pid))
		}
		args = append(args, "--process="+strings.Join(pidStrs, ","))
	}
	b, err := testexec.CommandContext(ctx, "cros-health-tool", args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "command failed")
	}

	// Log output to file for debugging.
	path := filepath.Join(outDir, "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		return nil, errors.Wrapf(err, "failed to write output to %s", path)
	}

	return b, nil
}

// RunAndParseTelem runs RunTelem and parses the CSV output into a
// two-dimensional array. An error is returned if there is a failure to obtain
// or parse the output or if a line of output has an unexpected number of
// fields.
func RunAndParseTelem(ctx context.Context, params TelemParams, outDir string) ([][]string, error) {
	b, err := RunTelem(ctx, params, outDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run telem command")
	}

	records, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse output [%q]", b)
	}

	return records, nil
}

// RunAndParseJSONTelem runs RunTelem and parses the JSON output.
// Example:
//
//	var result certainStruct
//	err := RunAndParseJSONTelem(_, _, _, &result)
func RunAndParseJSONTelem(ctx context.Context, params TelemParams, outDir string, result interface{}) error {
	b, err := RunTelem(ctx, params, outDir)
	if err != nil {
		return errors.Wrap(err, "failed to run telem command")
	}

	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()

	if err := dec.Decode(result); err != nil {
		return errors.Wrapf(err, "failed to decode data [%q]", b)
	}

	return nil
}
