// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
)

// RAPLValues represents the Intel "Running Average Power Limit" (RAPL) values.
// For further info read: https://www.kernel.org/doc/Documentation/power/powercap/powercap.txt
type RAPLValues struct {
	// Package0 contains the joules from Zone 0 in RAPL, which is about the sum of subzones Core, Uncore and DRAM.
	Package0 float64
	// Core contains the the joules from the CPU. It belongs to Package0.
	Core float64
	// Uncore contains the joules from the GPU. It belongs to Package0.
	Uncore float64
	// DRAM contains the joules from memory. It belongs to Package0.
	DRAM float64
	// Psys contains the joules from Zone 1 in RAPL.
	Psys float64
}

// ReportPerfMetrics appends to perfValues all the RAPL values.
// prefix is an optional string what will be used in perf.Metric Name.
func (rapl *RAPLValues) ReportPerfMetrics(perfValues *perf.Values, prefix string) {
	for _, e := range []struct {
		name  string
		value float64
	}{
		{"Package0", rapl.Package0},
		{"Core", rapl.Core},
		{"Uncore", rapl.Uncore},
		{"DRAM", rapl.DRAM},
		{"Psys", rapl.Psys},
	} {
		perfValues.Append(perf.Metric{
			Name:      prefix + e.name,
			Unit:      "joules",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, e.value)
	}
}

// RAPLSnapshot represents a snapshot of the RAPL values.
// It contains the RAPL values plus other variables needed to make the "diff" more efficient.
type RAPLSnapshot struct {
	// start contains a snapshot of the RAPL values.
	start *RAPLValues

	// maxJoules represents the max value that can be represented in RAPL before overflowing.
	// All Package0, Core, Uncore, DRAM and Psys have the same max joules value. So it is safe
	// to just get the max value for one of them, and use them for the rest.
	// The unit used is joules.
	maxJoules float64

	// dirsToParse represents the directories that should be parsed to get the RAPL values.
	// This is used as a cache, to avoid calculating the directories again while doing the diff.
	dirsToParse []string
}

// NewRAPLSnapshot returns a RAPLSnapshot.
// If no rapl files can be found, it returns a nil RAPLSnapshot, but does not return an error.
func NewRAPLSnapshot() (*RAPLSnapshot, error) {
	const (
		powercapDir       = "/sys/class/powercap"
		intelRAPLPrefix   = "intel-rapl:"
		raplMaxEnergyFile = "max_energy_range_uj"
	)
	_, err := os.Stat(powercapDir)
	if err != nil {
		if os.IsNotExist(err) {
			// TODO(crbug/1022205): Only Intel CPUs are supported at the moment.
			// Not considered an error if powercap is not present. ARM devices don't have it.
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to detect %q directory", powercapDir)
	}

	var dirsToParse []string
	if err := filepath.Walk(powercapDir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, intelRAPLPrefix) {
			dirsToParse = append(dirsToParse, path)
		}
		return nil
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to walk %q directory", powercapDir)
	}

	// TODO(crbug/1022205): Only Intel CPUs are supported at the moment.
	// Not considered an error if there are no intel-rapl files. AMD devices contain an empty folder.
	if len(dirsToParse) == 0 {
		return nil, nil
	}

	var rapl RAPLSnapshot
	rapl.dirsToParse = dirsToParse

	// It is safe to read max joules from the first directory, and reuse it for all the RAPL values.
	maxJoulesPath := path.Join(rapl.dirsToParse[0], raplMaxEnergyFile)
	rapl.maxJoules, err = readRAPLEnergy(maxJoulesPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse")
	}

	rapl.start, err = readRAPLValues(rapl.dirsToParse)
	if err != nil {
		return nil, errors.Wrap(err, "could read RAPL zones")
	}

	return &rapl, nil
}

// DiffWithCurrentRAPL returns the joules used since the snapshot was taken. If
// resetStart is true, then start is updated with the current values so that
// subsequent calls return a diff from now.
func (r *RAPLSnapshot) DiffWithCurrentRAPL(resetStart bool) (*RAPLValues, error) {
	end, err := readRAPLValues(r.dirsToParse)
	if err != nil {
		return nil, err
	}

	var ret RAPLValues
	ret.Package0 = diffJoules(r.start.Package0, end.Package0, r.maxJoules)
	ret.Core = diffJoules(r.start.Core, end.Core, r.maxJoules)
	ret.Uncore = diffJoules(r.start.Uncore, end.Uncore, r.maxJoules)
	ret.DRAM = diffJoules(r.start.DRAM, end.DRAM, r.maxJoules)
	ret.Psys = diffJoules(r.start.Psys, end.Psys, r.maxJoules)

	if resetStart {
		r.start = end
	}
	return &ret, nil
}

// diffJoules returns the joules used from "start" to "end" taking into account possible overflows.
func diffJoules(start, end, max float64) float64 {
	// It is theoretical possible to haveOverflow ?
	if start > end {
		return (max - start) + end
	}
	return end - start
}

// readRAPLValues reads the RAPL files contained in dirsToParse and returns the RAPL values contained in those files.
func readRAPLValues(dirsToParse []string) (*RAPLValues, error) {
	const (
		raplNameFile   = "name"
		raplEnergyFile = "energy_uj"
	)
	var rapl RAPLValues
	for _, dir := range dirsToParse {

		// Get RAPL name.
		name, err := readRAPLFile(path.Join(dir, raplNameFile))
		if err != nil {
			return nil, err
		}

		e, err := readRAPLEnergy(path.Join(dir, raplEnergyFile))
		if err != nil {
			return nil, err
		}

		switch name {
		case "package-0":
			rapl.Package0 = e
		case "dram":
			rapl.DRAM = e
		case "core":
			rapl.Core = e
		case "uncore":
			rapl.Uncore = e
		case "psys":
			rapl.Psys = e
		default:
			return nil, errors.Errorf("unexpected RAPL name: %q", name)
		}
	}
	return &rapl, nil
}

// readRAPLFile returns a string with the contents of the first line of the file.
// The returned string has all whitespaces from the beginning and end removed.
func readRAPLFile(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open file %q", filepath)
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	l, err := rd.ReadString('\n')
	if err != nil {
		return "", errors.Wrap(err, "failed to read string from file")
	}
	return strings.TrimSpace(l), nil
}

// readRAPLEnergy returns energy value, in joules units, contained in filepath.
func readRAPLEnergy(filepath string) (float64, error) {
	s, err := readRAPLFile(filepath)
	if err != nil {
		return 0, err
	}
	uj, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}

	// RAPL reports energy in micro joules. Micro joules gives us too much resolution,
	// and can make it a bit difficult to read. We use joules instead, gaining readability
	// the cost of losing some precision that we don't need.
	return float64(uj) / 1000., nil
}
