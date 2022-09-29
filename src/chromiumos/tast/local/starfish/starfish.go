// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package starfish provides functions for testing starfish module.
package starfish

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const defaultTimeout = 30 * time.Second
const noSimIndex = -1

// SfRegion enum corresponds to the regions where Starfish maybe deployed.
type SfRegion int

// Possible entries of SfRegion enum.
const (
	SfrXx SfRegion = iota
	SfrUs
	SfrUk
	SfrJp
	SfrCa
	SfrIn
)

// MaxSimSlots is max the number of SIM slots a Starfish module supports: 8
const MaxSimSlots = 8

// SimSlotData holds all the information about each SimSlot
type SimSlotData struct {
	sfCarrierName string
	isSimPresent  bool
}

// Starfish contains data pertaining to the current state, SIM selected, serial port, etc
type Starfish struct {
	region  SfRegion
	devid   string
	index   int
	simData [MaxSimSlots]SimSlotData
	sp      *SerialPort
}

// Setup finds and initializes the Starfish module.
func (s *Starfish) Setup(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish setup")
	devName, err := FindStarfish(ctx)
	if err != nil {
		return err
	}
	var sf SerialPort
	err = sf.Open(ctx, devName)
	if err != nil {
		return err
	}
	err = sf.Flush(ctx)
	if err != nil {
		return err
	}
	err = s.SimEject(ctx)
	if err != nil {
		return err
	}
	s.sp = &sf
	return nil
}

// DeviceID reads the DeviceID of the Starfish module.
func (s *Starfish) DeviceID(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish device id")
	resp, err := s.sp.SendCommand(ctx, "id")
	if err != nil {
		return err
	}
	var stub string = "Device ID: "
	if !strings.HasPrefix(resp, stub) {
		return errors.Errorf("invalid DeviceID response received: %s", resp)
	}
	s.devid = strings.TrimPrefix(resp, stub)
	testing.ContextLog(ctx, "device id: ", s.devid)
	return nil
}

// SimStatus reads the current status of the Starfish module.
func (s *Starfish) SimStatus(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish sim status")
	_, err := s.sp.SendCommand(ctx, "sim status")
	if err != nil {
		return err
	}
	return nil
}

// SimInsert emulates insertion of SIM into slot n (0 - 7)
func (s *Starfish) SimInsert(ctx context.Context, n int) error {
	testing.ContextLog(ctx, "starfish sim connect ", n)
	if n < 0 || n > 7 {
		return errors.Errorf("invalid sim slot index: %d", n)
	}
	if s.index != noSimIndex {
		testing.ContextLog(ctx, "ejecting active sim first")
		if err := s.SimEject(ctx); err != nil {
			return err
		}
	}
	var command string = "sim connect "
	command += strconv.Itoa(n + 1)
	_, err := s.sp.SendCommand(ctx, command)
	if err != nil {
		return err
	}
	s.index = n
	return nil
}

// SimEject emulates ejection of the active SIM slot
func (s *Starfish) SimEject(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish sim eject")
	_, err := s.sp.SendCommand(ctx, "sim eject")
	if err != nil {
		return err
	}
	s.index = noSimIndex
	return nil
}

// Teardown handles the close of the module
func (s *Starfish) Teardown(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish teardown")
	return s.sp.Close(ctx)
}
