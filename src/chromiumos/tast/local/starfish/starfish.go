// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package starfish provides functions for testing starfish module.
package starfish

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Type values defined in dbus-constants.h
// The values are used both for Service type and Technology type.
const (
	SimStr     = "SIM "
	EjectResp  = "Disabled SIM MUX"
	InsertResp = "Enabled SIM MUX:"
	DevIDResp  = "Device ID: "
	FoundStr   = "Found"
	NoneStr    = "None"
)

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

var exists = struct{}{}

// MaxSimSlots is max the number of SIM slots a Starfish module supports: 8
const MaxSimSlots = 8

// Starfish contains data pertaining to the current state, SIM selected, serial port, etc
type Starfish struct {
	sp       SerialInterface
	region   SfRegion
	devid    string
	index    int
	simSlots map[int]struct{}
}

// Setup finds and initializes the Starfish module.
func (s *Starfish) Setup(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish setup")
	var sf SerialInterface
	err := sf.Open(ctx)
	if err != nil {
		return err
	}
	s.sp = sf
	err = s.sp.Flush(ctx)
	if err != nil {
		return err
	}
	s.sp.SendCommand(ctx, "")
	err = s.DeviceID(ctx)
	if err != nil {
		return err
	}
	_, err = s.SimStatus(ctx)
	if err != nil {
		return err
	}
	return s.SimEject(ctx)
}

// DeviceID reads the DeviceID of the Starfish module.
func (s *Starfish) DeviceID(ctx context.Context) error {
	resp, err := s.sp.SendCommand(ctx, "id")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp[0], DevIdResp) {
		return errors.Errorf("invalid response: %s", resp)
	}
	s.devid = strings.TrimPrefix(resp[0], DevIdResp)
	testing.ContextLog(ctx, "device id: ", s.devid)
	return nil
}

// SimStatus queries and indicates the list of populated SIM slots, [0-7]
func (s *Starfish) SimStatus(ctx context.Context) ([]int, error) {
	responses, err := s.sp.SendCommand(ctx, "sim status")
	if err != nil {
		return nil, err
	}
	if len(responses) != MaxSimSlots {
		return nil, errors.Errorf("invalid response length: %s", responses)
	}
	var list []int
	for i := 0; i < MaxSimSlots; i++ {
		pref := SimStr + strconv.Itoa(i) + " = "
		if !strings.HasPrefix(responses[i], pref) {
			return nil, errors.Errorf("invalid response: %s", responses)
		}
		x := strings.TrimPrefix(responses[i], pref)
		if x == FoundStr {
			list = append(list, i)
		} else if x != NoneStr {
			return nil, errors.Errorf("invalid response: %s", responses)
		}
	}
	testing.ContextLog(ctx, "sims found: ", list)
	s.simSlots = make(map[int]struct{})
	for _, i := range list {
		s.simSlots[i] = exists
	}
	return list, nil
}

// SimInsert emulates insertion of SIM into slot n [0-7]
func (s *Starfish) SimInsert(ctx context.Context, n int) error {
	if n < 0 || n > 7 {
		return errors.Errorf("invalid sim slot index: %d", n)
	}
	if _, ok := s.simSlots[n]; !ok {
		return errors.Errorf("inactive sim slot index: %d", n)
	}
	if s.index == n {
		testing.ContextLog(ctx, "sim already inserted ", n)
		return nil
	}
	if s.index != noSimIndex {
		testing.ContextLog(ctx, "ejecting active sim first")
		if err := s.SimEject(ctx); err != nil {
			return err
		}
	}
	var command string = "sim connect "
	nplusone := strconv.Itoa(n + 1)
	command += nplusone
	responses, err := s.sp.SendCommand(ctx, command)
	if err != nil {
		return err
	}
	if !strings.Contains(responses[0], InsertResp+nplusone) {
		return errors.Errorf("invalid response: %s", responses)
	}
	s.index = n
	testing.ContextLog(ctx, "sim inserted ", n)
	return nil
}

// SimEject emulates ejection of the active SIM slot
func (s *Starfish) SimEject(ctx context.Context) error {
	if s.index == noSimIndex {
		testing.ContextLog(ctx, "sim already ejected ")
		return nil
	}
	responses, err := s.sp.SendCommand(ctx, "sim eject")
	if err != nil {
		return err
	}
	if (responses[0] != "") && (!strings.Contains(responses[0], EjectResp)) {
		return errors.Errorf("invalid response: %s", responses)
	}
	s.index = noSimIndex
	if responses[0] == "" {
		testing.ContextLog(ctx, "No sim present to be ejected")
	} else {
		testing.ContextLog(ctx, "sim ejected")
	}
	return nil
}

// Teardown handles the close of the module
func (s *Starfish) Teardown(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish teardown")
	return s.sp.Close(ctx)
}

// ActiveSimSlot indicates the current active SIM slot, [0-7], -1 indicates no active SIM.
func (s *Starfish) ActiveSimSlot(ctx context.Context) (int, bool) {
	return s.index, s.index == noSimIndex
}

// AvailableSimSlots indicates the cached list of populated SIM slots, [0-7] and active SIM, -1 indicates no active SIM.
func (s *Starfish) AvailableSimSlots(ctx context.Context) ([]int, int) {
	l := make([]int, 0, MaxSimSlots)
	for i := range s.simSlots {
		l = append(l, i)
	}
	sort.Ints(l)
	return l, s.index
}
