// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils used to do some component excution function.
package utils

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// VerifyPowerStatus verifies battery is charging or discharging.
func VerifyPowerStatus(ctx context.Context, dut *dut.DUT, isBatteryCharging bool) error {
	regex := `state:(\s+\w+\s?\w+)`
	expMatch := regexp.MustCompile(regex)

	out, err := dut.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve power supply info from DUT")
	}

	matches := expMatch.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return errors.Errorf("failed to match regex %q in %q", expMatch, string(out))
	}

	var chargingState bool
	if strings.TrimSpace(matches[1]) != "Discharging" {
		chargingState = true
	} else {
		chargingState = false
	}
	if chargingState != isBatteryCharging {
		return errors.Errorf("unexpected power state, got %t, want %t", chargingState, isBatteryCharging)
	}
	return nil
}
