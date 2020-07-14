// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package parameterize

import (
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type TestVal struct {
	SetupOption setup.BatteryDischargeMode
	Val         interface{}
}

func Parameterize(srcParams []testing.Param) []testing.Param {
	var dstParams []testing.Param
	for _, param := range srcParams {
		// Create parameterized test with given param
		// and hwdep.ForceDischarge()
		withBatteryMetrics := param
		withBatteryMetrics.ExtraHardwareDeps = hwdep.Merge(
			param.ExtraHardwareDeps,
			hwdep.D(hwdep.ForceDischarge()))
		withBatteryMetrics.Val = TestVal{
			SetupOption: setup.ForceBatteryDischarge,
			Val:         param.Val,
		}
		dstParams = append(dstParams, withBatteryMetrics)
		// Create parameterized test with given param
		// and hwdep.NoForceDischarge()
		withoutBatteryMetrics := param
		if len(param.Name) > 0 {
			withoutBatteryMetrics.Name =
				param.Name + "_nobatterymetrics"
		} else {
			withoutBatteryMetrics.Name =
				"nobatterymetrics"
		}
		withoutBatteryMetrics.ExtraHardwareDeps = hwdep.Merge(
			param.ExtraHardwareDeps,
			hwdep.D(hwdep.NoForceDischarge()))
		withoutBatteryMetrics.Val = TestVal{
			SetupOption: setup.NoBatteryDischarge,
			Val:         param.Val,
		}
		dstParams = append(dstParams, withoutBatteryMetrics)
	}
	return dstParams
}
