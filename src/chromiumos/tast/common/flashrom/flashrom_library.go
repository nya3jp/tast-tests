// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package flashrom is Flashrom Tast library to be used by all Tast tests.
package flashrom

import (
	"context"
)

// Programmer is Flashrom programmer, one of the values below
type Programmer int

// Supported programmers
const (
	dummyflasher Programmer = iota
	ec
	ft2232spi
	host
	raidenDebugSpi
)

// VerbosityLevel is Flashrom native logging verbosity level, one of the values below
type VerbosityLevel int

// Supported verbosity levels, corresponding to the levels defined in libflashrom.h
const (
	flashromMsgInfo   VerbosityLevel = iota // FLASHROM_MSG_INFO (not verbose)
	flashromMsgDebug                        // FLASHROM_MSG_DEBUG (a little verbose)
	flashromMsgDebug2                       // FLASHROM_MSG_DEBUG2 (medium verbosity)
	flashromMsgSpew                         // FLASHROM_MSG_SPEW (high verbosity)
)

// CommonParams are common parameters required for all library methods
type CommonParams struct {
	verbosity       VerbosityLevel
	programmer      Programmer
	programmerParam string
	// Tast-specific log level ?
}

// ModificationParams are additional parameters for reading and WP operations
type ModificationParams struct {
	fileName   string
	regionName string // optional
}

// WriteParams are additional parameters for write operations
type WriteParams struct {
	noverifyAll        bool
	noverify           bool // used as extra args in fw-config.json
	flashcontentsImage string
}

// Probe probes the chip
func Probe(ctx context.Context, commonParams CommonParams) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPStatus requests software write-protect status of the chip
func WPStatus(ctx context.Context, commonParams CommonParams) (bool, error) {
	// TODO b:247668196 implement

	return true, nil
}

// ProbeAndWPStatus probes and requests software write-protect status of the chip
func ProbeAndWPStatus(ctx context.Context, commonParams CommonParams) (bool, error) {
	// TODO b:247668196 implement

	return true, nil
}

// Read reads the chip
func Read(ctx context.Context, commonParams CommonParams, modParams ModificationParams) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// ProbeAndRead probes and reads the chip
func ProbeAndRead(ctx context.Context, commonParams CommonParams, modParams ModificationParams) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// ProbeAndWPRegion probes and sets software write-protect reqion on the chip
func ProbeAndWPRegion(ctx context.Context, commonParams CommonParams, modParams ModificationParams, wpRegionName string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// ProbeAndWPToggle probes and toggles software write-protect on the chip.
// Enables write-protect if enable parameter is true, disables otherwise.
func ProbeAndWPToggle(ctx context.Context, commonParams CommonParams, modParams ModificationParams, enable bool) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// ProbeAndWPEnableAndWPRange probes, enables software write-protect and sets write-protect range on the chip
func ProbeAndWPEnableAndWPRange(ctx context.Context, commonParams CommonParams, modParams ModificationParams, wpRange string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// Write writes on chip
func Write(ctx context.Context, commonParams CommonParams, modParams ModificationParams, writeParams WriteParams) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// ProbeAndWrite probes and writes on chip
func ProbeAndWrite(ctx context.Context, commonParams CommonParams, modParams ModificationParams, writeParams WriteParams) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}
