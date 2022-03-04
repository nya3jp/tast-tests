// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// PowerTestParam is a required part of the type of Val on the arguments to
// PowerTestParams. Specifically, that Val type must be a pointer to either
// PowerTestParam or a struct that embeds PowerTestParam.
type PowerTestParam BatteryDischargeMode

// PowerTestParams returns parameters of tests that only run on devices
// supporting forced battery discharge, and corresponding nobatterymetrics
// variants which only run on devices that do not support forced battery
// discharge. Val has ForceBatteryDischarge on the tests without the added
// nobatterymetrics suffix, and NoBatteryDischarge on the nobatterymetrics
// variants. See PowerTestParam for details on the type of Val.
func PowerTestParams(params ...testing.Param) []testing.Param {
	paramsWithBatteryDischarge := make([]testing.Param, len(params))
	paramsWithoutBatteryDischarge := make([]testing.Param, len(params))
	for i, param := range params {
		paramsWithBatteryDischarge[i] = paramWithBatteryDischarge(param)
		paramsWithoutBatteryDischarge[i] = paramWithoutBatteryDischarge(param)
	}
	return append(paramsWithBatteryDischarge, paramsWithoutBatteryDischarge...)
}

// paramWithBatteryDischarge arranges for a test to only run on devices
// supporting forced battery discharge, with ForceBatteryDischarge on Val.
func paramWithBatteryDischarge(param testing.Param) testing.Param {
	result := param
	result.ExtraHardwareDeps = hwdep.Combine(param.ExtraHardwareDeps, hwdep.D(hwdep.ForceDischarge()))
	result.Val.(powerTestParamInterface).set(PowerTestParam(ForceBatteryDischarge))
	return result
}

// paramWithoutBatteryDischarge arranges for a test to only run on devices
// not supporting forced battery discharge, with NoBatteryDischarge on Val.
// The nobatterymetrics suffix is added to the name.
func paramWithoutBatteryDischarge(param testing.Param) testing.Param {
	result := param
	result.Name = addNoBatteryMetricsSuffix(param.Name)
	result.ExtraHardwareDeps = hwdep.Combine(param.ExtraHardwareDeps, hwdep.D(hwdep.NoForceDischarge()))
	result.Val.(powerTestParamInterface).set(PowerTestParam(NoBatteryDischarge))
	return result
}

// powerTestParamInterface is implemented by *PowerTestParam, for use by
// PowerTestParams. If the type of Val is a pointer to a struct that embeds
// PowerTestParam, it will inherit the implementation of this interface.
type powerTestParamInterface interface {
	// set sets the PowerTestParam value.
	set(value PowerTestParam)
}

func (param *PowerTestParam) set(value PowerTestParam) {
	*param = value
}

// addNoBatteryMetricsSuffix adds the nobatterymetrics suffix to a name.
func addNoBatteryMetricsSuffix(name string) string {
	if name == "" {
		return "nobatterymetrics"
	}
	return name + "_nobatterymetrics"
}
