// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package starfish provides functions for testing starfish module.
package starfish

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Various string used to parse responses
const (
	simStr     = "SIM "
	ejectResp  = "Disabled SIM MUX"
	insertResp = "Enabled SIM MUX:"
	devIDResp  = "Device ID: "
	foundStr   = "Found"
	noneStr    = "None"
)

// NoSimIndex is returned if none of the SIMs is actively connected
const NoSimIndex = -1

var exists = struct{}{}

// MaxSimSlots is max the number of SIM slots a Starfish module supports: 8
const MaxSimSlots = 8

// Starfish contains data pertaining to the current state, SIM selected, serial port, etc
type Starfish struct {
	sp       shim
	devid    string
	index    int
	simSlots map[int]struct{}
}

// Setup initializes the Starfish module.
func (s *Starfish) Setup(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish setup")
	var sf shim
	logs, err := sf.Init(ctx)
	s.printLogs(ctx, logs)
	if err != nil {
		return err
	}
	s.sp = sf
	err = s.deviceID(ctx)
	if err != nil {
		return err
	}
	err = s.simStatus(ctx)
	if err != nil {
		return err
	}
	err = s.SimEject(ctx)
	if err != nil {
		return err
	}
	carrierInfo := "2_ATT"
	values := strings.Split(carrierInfo, "_")
	if len(values) != 2 {
		return errors.Errorf("failed to parse carrier info: %s", carrierInfo)
	}
	v, err := strconv.Atoi(values[0])
	if err != nil {
		return errors.Errorf("failed to parse slotIndex: %s", values[0])
	}
	if err := s.SimInsert(ctx, v); err != nil {
		return errors.Errorf("failed sim insert command: %s", err)
	}
	testing.ContextLog(ctx, "Inserted SIM for carrier: ", values[1], " in slot: ", v)
	return nil

}

// deviceID reads the DeviceID of the Starfish module.
func (s *Starfish) deviceID(ctx context.Context) error {
	responses, logs, err := s.sp.SendCommand(ctx, "id")
	s.printLogs(ctx, logs)
	if err != nil {
		return err
	}
	if len(responses) < 1 {
		return errors.New("invalid response")
	}
	if !strings.HasPrefix(responses[0], devIDResp) {
		return errors.Errorf("invalid response: %s", responses)
	}
	s.devid = strings.TrimPrefix(responses[0], devIDResp)
	testing.ContextLog(ctx, "device id: ", s.devid)
	return nil
}

// simStatus queries and indicates the list of populated SIM slots, [0-7]
func (s *Starfish) simStatus(ctx context.Context) error {
	responses, logs, err := s.sp.SendCommand(ctx, "sim status")
	s.printLogs(ctx, logs)
	if err != nil {
		return err
	}
	if len(responses) != MaxSimSlots {
		return errors.Errorf("invalid response length: %s", responses)
	}
	var list []int
	for i := 0; i < MaxSimSlots; i++ {
		pref := simStr + strconv.Itoa(i) + " = "
		if !strings.HasPrefix(responses[i], pref) {
			return errors.Errorf("invalid response: %s", responses)
		}
		x := strings.TrimPrefix(responses[i], pref)
		if x == foundStr {
			list = append(list, i)
		} else if x != noneStr {
			return errors.Errorf("invalid response: %s", responses)
		}
	}
	testing.ContextLog(ctx, "sims found: ", list)
	s.simSlots = make(map[int]struct{})
	for _, i := range list {
		s.simSlots[i] = exists
	}
	return nil
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
	if s.index != NoSimIndex {
		testing.ContextLog(ctx, "ejecting active sim first")
		if err := s.SimEject(ctx); err != nil {
			return err
		}
	}
	var command string = fmt.Sprintf("sim connect %d", n+1)
	nplusone := strconv.Itoa(n + 1)
	command += nplusone
	responses, logs, err := s.sp.SendCommand(ctx, command)
	s.printLogs(ctx, logs)
	if err != nil {
		return err
	}
	if len(responses) < 1 {
		return errors.New("invalid response")
	}
	if !strings.Contains(responses[0], fmt.Sprintf("%s%d", insertResp, n+1)) {
		return errors.Errorf("invalid response: %s", responses)
	}
	s.index = n
	testing.ContextLog(ctx, "sim inserted ", n)
	return nil
}

// SimEject emulates ejection of the active SIM slot
func (s *Starfish) SimEject(ctx context.Context) error {
	if s.index == NoSimIndex {
		testing.ContextLog(ctx, "sim already ejected ")
		return nil
	}
	responses, logs, err := s.sp.SendCommand(ctx, "sim eject")
	s.printLogs(ctx, logs)
	if err != nil {
		return err
	}
	if len(responses) < 1 {
		s.index = NoSimIndex
		testing.ContextLog(ctx, "Warning, empty response received")
		return nil
	}
	if (responses[0] != "") && (!strings.Contains(responses[0], ejectResp)) {
		return errors.Errorf("invalid response: %s", responses[0])
	}
	t := s.index
	s.index = NoSimIndex
	if responses[0] == "" {
		testing.ContextLog(ctx, "No sim present to be ejected")
	} else {
		testing.ContextLog(ctx, "sim ejected ", t)
	}
	return nil
}

// Teardown handles the close of the module
func (s *Starfish) Teardown(ctx context.Context) error {
	testing.ContextLog(ctx, "starfish teardown")
	if err := s.SimEject(ctx); err != nil {
		testing.ContextLog(ctx, "Failed sim eject command: ", err)
	}
	return s.sp.Close(ctx)
}

// ActiveSimSlot indicates the current active SIM slot, [0-7], -1 indicates no active SIM.
func (s *Starfish) ActiveSimSlot(ctx context.Context) (int, bool) {
	return s.index, s.index == NoSimIndex
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

// printLogs prints logs from the Starfish module
func (s *Starfish) printLogs(ctx context.Context, logs []string) {
	if logs == nil {
		return
	}
	for _, line := range logs {
		testing.ContextLog(ctx, "------", line)
	}
}
