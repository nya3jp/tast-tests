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

// Params are common parameters required for all library methods
type Params struct {
	verbosity       VerbosityLevel
	programmer      Programmer
	programmerParam string
	fileName        string
	regionName      string // optional
}

// Probe probes the chip
func Probe(ctx context.Context, params Params) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPStatus probes and requests software write-protect status of the chip
func WPStatus(ctx context.Context, params Params) (bool, error) {
	// TODO b:247668196 implement

	return true, nil
}

// Read probes and reads the chip
func Read(ctx context.Context, params Params) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPRegion probes and sets software write-protect reqion on the chip
func WPRegion(ctx context.Context, params Params, wpRegionName string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPToggle probes and toggles software write-protect on the chip.
// Enables write-protect if enable parameter is true, disables otherwise.
func WPToggle(ctx context.Context, params Params, enable bool) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPEnableAndWPRange probes, enables software write-protect and sets write-protect range on the chip
func WPEnableAndWPRange(ctx context.Context, params Params, wpRange string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// Write probes and writes on chip
func Write(ctx context.Context, params Params, noverifyAll, noverify bool, flashcontentsImage string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}
