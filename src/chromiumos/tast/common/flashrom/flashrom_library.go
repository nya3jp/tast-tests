// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/*
Package flashrom is Flashrom Tast library to be used by all Tast tests.

Instructions:
1) Use the Builder to construct the Instance of flashrom.
2) Run Probe() method on the Builder to complete initialisation of your instance
and probe the chip. Probe() method returns Instance which is ready to use.
3) Invoke methods of your Instance to run flashrom operations.
4) Shutdown your instance at the end.

Sample usage:

Instance instance := Builder
	.FlashromInit(VerbosityLevel.flashromMsgDebug)
	.ProgrammerInit(Programmer.host, "")
	.Probe()

retCode, err := instance.Read(ctx, "tmp/dump.bin")

instance.FullShutdown()
*/
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
	regionNames     string // optional
}

// Builder builds flashrom instance
type Builder struct {
	params Params
}

// Instance of flashrom to run operations
type Instance struct {
	params Params
}

// FlashromInit sets verbosity level
func (b *Builder) FlashromInit(verbosity VerbosityLevel) (*Builder, error) {
	// TODO b:247668196 implement

	return nil, nil
}

// SelectLayoutRegions selects layout regions to operate on
func (b *Builder) SelectLayoutRegions(regionsList []string) (*Builder, error) {
	// TODO b:247668196 implement

	return nil, nil
}

// ProgrammerInit initialises flashrom programmer with given params
func (b *Builder) ProgrammerInit(programmer Programmer, programmerParams string) (*Builder, error) {
	// TODO b:247668196 implement

	return nil, nil
}

// Probe probes the chip
func (b *Builder) Probe(ctx context.Context) (*Instance, error) {
	// TODO b:247668196 implement

	return nil, nil
}

// WPStatus probes and requests software write-protect status of the chip
func (i *Instance) WPStatus(ctx context.Context) (bool, error) {
	// TODO b:247668196 implement

	return true, nil
}

// Read probes and reads the chip
func (i *Instance) Read(ctx context.Context, filePath string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPRegion probes and sets software write-protect reqion on the chip
func (i *Instance) WPRegion(ctx context.Context, wpRegionName string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPToggle probes and toggles software write-protect on the chip.
// Enables write-protect if enable parameter is true, disables otherwise.
func (i *Instance) WPToggle(ctx context.Context, enable bool) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// WPEnableWithRange probes, enables software write-protect and sets write-protect range on the chip
func (i *Instance) WPEnableWithRange(ctx context.Context, wpRange string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// Write probes and writes on chip
func (i *Instance) Write(ctx context.Context, filePath string, noverifyAll, noverify bool, flashcontentsImage string) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}

// FullShutdown shuts down flashrom programmer, cleans up all resources and shuts down flashrom
func (i *Instance) FullShutdown(ctx context.Context) (int, error) {
	// TODO b:247668196 implement

	return 0, nil
}
