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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

var isZoneExpected = map[string]bool{
	// core reports the the joules from the CPU. It belongs to package-0.
	"core": true,
	// dram reports the joules from memory. It belongs to package-0.
	"dram": true,
	// package-0 reports the joules from Zone 0 in RAPL, which is about the sum of subzones core, uncore and dram.
	"package-0": true,
	// psys reports the joules from Zone 1 in RAPL.
	"psys": true,
	// uncore reports the joules from the GPU. It belongs to package-0.
	"uncore": true,
}

// RAPLValues represents the Intel "Running Average Power Limit" (RAPL) values.
// For further info read: https://www.kernel.org/doc/Documentation/power/powercap/powercap.txt
type RAPLValues struct {
	// joules contains the joules from RAPL
	joules map[string]float64
}

func newRAPLValues() *RAPLValues {
	return &RAPLValues{make(map[string]float64)}
}

// ReportPerfMetrics appends to perfValues all the RAPL values.
// prefix is an optional string what will be used in perf.Metric Name.
func (rapl *RAPLValues) ReportPerfMetrics(perfValues *perf.Values, prefix string) {
	for _, e := range []struct {
		name  string
		value float64
	}{
		{"Package0", rapl.joules["package-0"]},
		{"Core", rapl.joules["core"]},
		{"Uncore", rapl.joules["uncore"]},
		{"DRAM", rapl.joules["dram"]},
		{"Psys", rapl.joules["psys"]},
	} {
		perfValues.Append(perf.Metric{
			Name:      prefix + e.name,
			Unit:      "joules",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, e.value)
	}
}

// Total returns the sum of joules at the top level.
func (rapl *RAPLValues) Total() float64 {
	return rapl.joules["package0"] + rapl.joules["psys"]
}

// Uncore returns the joules from the GPU.
func (rapl *RAPLValues) Uncore() float64 {
	return rapl.joules["uncore"]
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

// diffWithCurrentRAPL returns the joules used since the snapshot was taken. If
// resetStart is set, then the snapshot is updated with the current values so
// that the next call is relative to now.
func (r *RAPLSnapshot) diffWithCurrentRAPL(resetStart bool) (*RAPLValues, error) {
	end, err := readRAPLValues(r.dirsToParse)
	if err != nil {
		return nil, err
	}

	ret := newRAPLValues()
	for name, startValue := range r.start.joules {
		ret.joules[name] = diffJoules(startValue, end.joules[name], r.maxJoules)
	}
	if resetStart {
		r.start = end
	}
	return ret, nil
}

// DiffWithCurrentRAPL returns the joules used since the snapshot was taken.
func (r *RAPLSnapshot) DiffWithCurrentRAPL() (*RAPLValues, error) {
	return r.diffWithCurrentRAPL(false)
}

// DiffWithCurrentRAPLAndReset returns the joules used since the snapshot was
// taken. The current snapshot is updated so that the next diff will be relative
// to now.
func (r *RAPLSnapshot) DiffWithCurrentRAPLAndReset() (*RAPLValues, error) {
	return r.diffWithCurrentRAPL(true)
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
	rapl := newRAPLValues()
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

		if !isZoneExpected[name] {
			return nil, errors.Errorf("unexpected RAPL name: %q", name)
		}
		rapl.joules[name] = e
	}
	return rapl, nil
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
	return float64(uj) / 1000000., nil
}
