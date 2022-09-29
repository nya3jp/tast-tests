// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package starfish provides functions for testing starfish module.
package starfish

import (
	"context"
	"time"

	"github.com/google/gousb"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const defaultTimeout = 30 * time.Second

// Starfish vid and pid: Google.
const (
	StarfishVid = 0x18d1
	StarfishPid = 0x1234
)

// SfRegion enum corresponds to the regions where Starfish maybe deployed.
type SfRegion int

// Possible entries of SfRegion enum.
const (
	SfrXx SfRegion = iota
	SfrUs
	SfrUk
	SfrJp
	SfrCa
	SfrIi
)

// MaxSimSlots is max the number of SIM slots a Starfish module supports: 8
const MaxSimSlots = 8

// SimSlotIndex type to indicate the cardinal number of the SimSlot
type SimSlotIndex int

// Allowed SimSlotIndex enums
const (
	Ssi0 SimSlotIndex = iota
	Ssi1
	Ssi2
	Ssi3
	Ssi4
	Ssi5
	Ssi6
	// Append more indices here if applicable
	SsiE = MaxSimSlots - 1
)

// SimSlotData holds all the information about each SimSlot
type SimSlotData struct {
	SfCarrierName string
}

// ModuleInfo Gets static module info from the board.
type ModuleInfo struct {
	bsn    string
	region SfRegion
}

// Helper fetches Starfish module properties.
type Helper struct {
	moduleInfo ModuleInfo
	simSlots   map[SimSlotIndex][]SimSlotData
}

// NewHelper creates a Helper object and ensures that a Cellular Device is present.
func NewHelper(ctx context.Context) (*Helper, error) {
	ctx, st := timing.Start(ctx, "Helper.NewHelper")
	defer st.End()

	ctx1 := gousb.NewContext()
	defer ctx1.Close()

	// Open any device with a given VID/PID using a convenience function.
	dev, err := ctx1.OpenDeviceWithVIDPID(StarfishVid, StarfishPid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	defer dev.Close()

	// Claim the default interface using a convenience function.
	// The default interface is always #0 alt #0 in the currently active
	// config.
	intf, done, err := dev.DefaultInterface()
	if err != nil {
		return nil, errors.Wrap("%s.DefaultInterface(): %v", dev, err)
	}
	defer done()

	testing.ContextLog(ctx, "settings: ", intf.InterfaceSetting())

	/*
		// In this interface open endpoint #6 for reading.
		epIn, err := intf.InEndpoint(6)
		if err != nil {
			log.Fatalf("%s.InEndpoint(6): %v", intf, err)
		}

		// And in the same interface open endpoint #5 for writing.
		epOut, err := intf.OutEndpoint(5)
		if err != nil {
			log.Fatalf("%s.OutEndpoint(5): %v", intf, err)
		}
	*/
	helper := Helper{}
	return &helper, nil
}
