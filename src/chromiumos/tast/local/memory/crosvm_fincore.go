// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
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
		pathRE: regexp.MustCompile(`.*`),
		name:   "crosvm_file",
	},
}

type fincoreJSONEntry struct {
	Resident uint64 `json:"res"`
	Pages    uint64 `json:"pages"`
	Size     uint64 `json:"size"`
	File     string `json:"file"`
}

type fincoreJSON struct {
	Fincore []fincoreJSONEntry `json:"fincore"`
}

type fincoreJSONv1 struct {
	fincoreJSON
}

// UnmarshalJSON overrides the default JSON parsing to be compatible with an
// older version of fincore output.
func (f fincoreJSONv1) UnmarshalJSON(b []byte) error {
	var v1 struct {
		Fincore []struct {
			Resident uint64 `json:"res,string"`
			Pages    uint64 `json:"pages,string"`
			Size     uint64 `json:"size,string"`
			File     string `json:"file"`
		} `json:"fincore"`
	}
	if err := json.Unmarshal(b, &v1); err != nil {
		return err
	}
	f.Fincore = make([]fincoreJSONEntry, len(v1.Fincore))
	for i, e := range v1.Fincore {
		f.Fincore[i] = fincoreJSONEntry{
			Resident: e.Resident,
			Pages:    e.Pages,
			Size:     e.Size,
			File:     e.File,
		}
	}
	return nil
}

func parseFincoreJSON(ctx context.Context, bytes []byte) (*fincoreJSON, error) {
	var v2 fincoreJSON
	err2 := json.Unmarshal(bytes, &v2)
	if err2 == nil {
		return &v2, nil
	}

	var v1 fincoreJSONv1
	err1 := json.Unmarshal(bytes, &v1)
	if err1 == nil {
		return &(v1.fincoreJSON), nil
	}

	// Failure, log fincore output and the errors from the previous versions.
	testing.ContextLogf(ctx, "Failed to parse fincore output %q", string(bytes))
	testing.ContextLog(ctx, "Failed to parse with v1 format: ", err1)

	// Return the error from parsing the most recent format.
	return nil, errors.Wrap(err2, "failed to parse fincore output")
}

var fincoreArcVMDiskArgRe = regexp.MustCompile("^(--disk|--rwdisk|--root|--rwroot)$")

// CrosvmFincoreMetrics logs a JSON file with the amount resident memory for
// each file used as a disk by crosvm. If p is not nil, the amount of memory
// used by each VM type is logged as perf.Values. If outdir is "", then no logs
// are written.
func CrosvmFincoreMetrics(ctx context.Context, p *perf.Values, outdir, suffix string) error {
	// Look for crosvm processes with
	processes, err := process.Processes()
	const crosvmPath = "/usr/bin/crosvm"
	const conciergePath = "/usr/bin/vm_concierge"
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
		parent, err := p.Parent()
		if err != nil {
			return errors.Wrapf(err, "failed to get parent for process %d", p.Pid)
		}
		if exe, err := parent.Exe(); err != nil {
			// Some processes don't have a /proc/<pid>/exe, this process might
			// have terminated.
			continue
		} else if exe != conciergePath {
			// We only care about crosvm spawned by concierge as that will be the
			// top-level crosvm process. All forks that crosvm
			// spawns for each device will have same cmdline anyway but at the same
			// time not each will retain same FDs so we may end up with collisions.
			continue
		}
		for i, arg := range args {
			if fincoreArcVMDiskArgRe.MatchString(arg) {
				if i+1 >= len(args) {
					return errors.Errorf("crosvm has --disk arg with no path, args=%v", args)
				}
				file := strings.Split(args[i+1], ",")[0]
				if strings.HasPrefix(file, "/proc/self/fd") {
					file = strings.ReplaceAll(file, "self", strconv.Itoa(int(p.Pid)))
					file, err = os.Readlink(file)
					if err != nil {
						return errors.Wrapf(err, "failed to readlink %s", file)
					}
				}
				disks[file] = true
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
	testing.ContextLogf(ctx, "Collected fincore arguments: %+q", args)
	fincoreBytes, err := testexec.CommandContext(ctx, "fincore", args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to get fincore for %v", args)
	}
	if len(outdir) > 0 {
		filename := fmt.Sprintf("fincore%s.json", suffix)
		if err := ioutil.WriteFile(path.Join(outdir, filename), fincoreBytes, 0644); err != nil {
			return errors.Wrapf(err, "failed to write fincore JSON to %s", filename)
		}
	}

	if p == nil {
		return nil
	}

	fincore, err := parseFincoreJSON(ctx, fincoreBytes)
	if err != nil {
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
