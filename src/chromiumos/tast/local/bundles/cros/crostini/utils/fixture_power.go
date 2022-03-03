// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// StationPowerState Power state for station.
type StationPowerState int

// station power option
const (
	StationPowerOff StationPowerState = 0
	StationPowerOn  StationPowerState = 1
)

// String returns a human-readable string representation for type FixtureType.
func (s StationPowerState) String() string {
	switch s {
	case StationPowerOff:
		return "Power off"
	case StationPowerOn:
		return "Power on"
	default:
		return fmt.Sprintf("Unknown station power state: %d", s)
	}
}

// SetStationPower sets the station power by a given power state.refer to : power.SetDisplayPower
func SetStationPower(ctx context.Context, s *testing.State, want StationPowerState) error {

	s.Logf("%s station", want.String())

	// action: turn on / off
	var action string
	if want == StationPowerOn {
		action = "1" // turn on
	} else {
		action = "0" // turn on
	}

	// station port
	var stationPort string
	stationPort = "1"

	if err := AviosysControl(s, action, stationPort); err != nil {
		return errors.Wrap(err, "failed to execute aviosys control")
	}

	// wait for system response
	testing.Sleep(ctx, 10*time.Second)

	return nil
}
