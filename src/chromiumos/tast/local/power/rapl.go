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
)

// RAPLValues represents the Intel "Running Average Power Limit" (RAPL) values.
// For further info read: https://www.kernel.org/doc/Documentation/power/powercap/powercap.txt
type RAPLValues struct {
	// Package0 contains the micro-joules from Zone 0 in RAPL, which is about the sum of subzones Core, Uncore and DRAM.
	Package0 uint64
	// Core contains the the micro-joules from the CPU. It belongs to Package0.
	Core uint64
	// Uncore contains the micro-joules from the GPU. It belongs to Package0.
	Uncore uint64
	// DRAM contains the micro-joules from memory. It belongs to Package0.
	DRAM uint64
	// Psys contains the micro-joules from Zone 1 in RAPL.
	Psys uint64
}

// RAPLSnapshot represents a snapshot of the RAPL values.
// It contains the RAPL values plus other variables needed to make the "diff" more efficient.
type RAPLSnapshot struct {
	// start contains a snapshot of the RAPL values.
	start *RAPLValues

	// maxUJoules represents the max value that can be represented in RAPL before overflowing.
	// All Package0, Core, Uncore, DRAM and Psys have the same max micro-joules value. So it is safe
	// to just get the max value for one of them, and use them for the rest.
	// The unit used is micro joules.
	maxUJoules uint64

	// dirsToParse represents the directories that should be parsed to get the RAPL values.
	// This is used as a cache, to avoid calculating the directories again while doing the diff.
	dirsToParse []string
}

// NewRAPLSnapshot returns a RAPLSnapshot.
func NewRAPLSnapshot() (*RAPLSnapshot, error) {
	const (
		powercapDir       = "/sys/class/powercap"
		intelRAPLPrefix   = "intel-rapl:"
		raplMaxEnergyFile = "max_energy_range_uj"
	)
	_, err := os.Stat(powercapDir)
	if err != nil {
		if os.IsNotExist(err) {
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

	if len(dirsToParse) == 0 {
		// TODO(crbug/1022205): Add support for non-Intel CPUs.
		return nil, errors.New("could not find any Intel-RAPL directory, only Intel CPUs are supported")
	}

	var rapl RAPLSnapshot
	rapl.dirsToParse = dirsToParse

	// It is safe to read max joules from the first directory, and reuse it for all the RAPL values.
	maxJoulesPath := path.Join(rapl.dirsToParse[0], raplMaxEnergyFile)
	rapl.maxUJoules, err = readRAPLEnergy(maxJoulesPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse")
	}

	rapl.start, err = readRAPLValues(rapl.dirsToParse)
	if err != nil {
		return nil, errors.Wrap(err, "could read RAPL zones")
	}

	return &rapl, nil
}

// DiffWithCurrentRAPL returns the micro joules used since the snapshot was taken.
func (r *RAPLSnapshot) DiffWithCurrentRAPL() (*RAPLValues, error) {
	end, err := readRAPLValues(r.dirsToParse)
	if err != nil {
		return nil, err
	}

	var ret RAPLValues
	ret.Package0 = diffJoules(r.start.Package0, end.Package0, r.maxUJoules)
	ret.Core = diffJoules(r.start.Core, end.Core, r.maxUJoules)
	ret.Uncore = diffJoules(r.start.Uncore, end.Uncore, r.maxUJoules)
	ret.DRAM = diffJoules(r.start.DRAM, end.DRAM, r.maxUJoules)
	ret.Psys = diffJoules(r.start.Psys, end.Psys, r.maxUJoules)
	return &ret, nil
}

// diffJoules returns the microjoules used from "start" to "end" taking into account possible overflows.
func diffJoules(start, end, max uint64) uint64 {
	// Overflow ?
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

// readRAPLEnergy returns energy value, usually in micro-joules units, contained in filepath.
func readRAPLEnergy(filepath string) (uint64, error) {
	s, err := readRAPLFile(filepath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}
