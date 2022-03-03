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

// PSUState PSU Power state
type PSUState int

// station power option
const (
	PSUPowerOff PSUState = 0
	PSUPowerOn  PSUState = 1
)

// String returns a human-readable string representation for type PSUState.
func (s PSUState) String() string {
	switch s {
	case PSUPowerOff:
		return "Power off"
	case PSUPowerOn:
		return "Power on"
	default:
		return fmt.Sprintf("Unknown PSU power state: %d", s)
	}
}

// SetStationPower sets the station power by a given power state.refer to : power.SetDisplayPower
func SetStationPower(ctx context.Context, want PSUState) error {
	// station port
	var stationPort string
	stationPort = "1"

	if err := ControlAviosys(ctx, fmt.Sprintf("%d", want), stationPort); err != nil {
		return errors.Wrap(err, "failed to execute aviosys control")
	}

	// wait for system response
	testing.Sleep(ctx, 10*time.Second)

	return nil
}
