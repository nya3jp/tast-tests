// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

type diskCategory struct {
	pathRE *regexp.Regexp
	name   string
}

// diskCategories defines categories used to aggregate the fincore memory of
// files used as disks in crosvm.
var diskCategories = []diskCategory{
	{
		pathRE: regexp.MustCompile(`^/opt/google/vms/android/`),
		name:   "arcvm_file",
	}, {
		pathRE: regexp.MustCompile(`^/run/imageloader/cros-termina/`),
		name:   "crostini_file",
	}, {
		pathRE: regexp.MustCompile(`.*`),
		name:   "crosvm_other_file",
	},
}

// CrosvmFincoreMetrics logs a JSON file with the amount resident memory for
// each file used as a disk by crosvm. If p is not nil, the amount of memory
// used by each VM type is logged as perf.Values.
func CrosvmFincoreMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	// Look for crosvm processes with
	processes, err := process.Processes()
	const crosvmPath = "/usr/bin/crosvm"
	const diskArg = "--disk"
	disks := make(map[string]bool)
	for _, p := range processes {
		if exe, err := p.Exe(); err != nil {
			// Some processes don't have a /proc/<pid>/exe, this process might
			// have terminated.
			continue
		} else if exe != crosvmPath {
			// We only care about crosvm
			continue
		}
		args, err := p.CmdlineSlice()
		if err != nil {
			return errors.Wrapf(err, "failed to get arguments for process %d", p.Pid)
		}
		for i, arg := range args {
			if arg == diskArg {
				if i+1 >= len(args) {
					return errors.Errorf("crosvm has --disk arg with no path, args=%v", args)
				}
				disks[args[i+1]] = true
			}
		}
	}
	if len(disks) == 0 {
		return nil
	}
	args := []string{"--bytes", "--json"}
	for disk := range disks {
		args = append(args, disk)
	}
	fincoreBytes, err := testexec.CommandContext(ctx, "fincore", args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to get fincore for %v", args)
	}
	filename := fmt.Sprintf("fincore%s.json", suffix)
	if err := ioutil.WriteFile(path.Join(outdir, filename), fincoreBytes, 0644); err != nil {
		return errors.Wrapf(err, "failed to write fincore JSON to %s", filename)
	}

	if p == nil {
		return nil
	}

	var fincore struct {
		Fincore []struct {
			Resident uint64 `json:"res,string"`
			Pages    uint64 `json:"pages,string"`
			Size     uint64 `json:"size,string"`
			File     string `json:"file"`
		} `json:"fincore"`
	}
	if err := json.Unmarshal(fincoreBytes, &fincore); err != nil {
		return errors.Wrap(err, "failed to parse fincore JSON output")
	}

	metrics := make(map[string]float64)
	for _, file := range fincore.Fincore {
		for _, category := range diskCategories {
			if category.pathRE.MatchString(file.File) {
				metrics[category.name] += float64(file.Resident) / MiB
				break
			}
		}
	}
	for name, resident := range metrics {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("%s%s", name, suffix),
				Unit:      "MiB",
				Direction: perf.SmallerIsBetter,
			},
			resident,
		)
	}
	return nil
}
