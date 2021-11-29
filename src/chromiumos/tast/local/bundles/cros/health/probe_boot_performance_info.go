// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type bootPerformanceInfo struct {
	BootUpSeconds     float64 `json:"boot_up_seconds"`
	BootUpTimestamp   float64 `json:"boot_up_timestamp"`
	ShutdownSeconds   float64 `json:"shutdown_seconds"`
	ShutdownTimestamp float64 `json:"shutdown_timestamp"`
	ShutdownReason    string  `json:"shutdown_reason"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBootPerformanceInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for boot performance info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics",
			// TODO(b/213993701): Reenable after mapping cbmem.
			"no_manatee"},
		Fixture: "crosHealthdRunning",
	})
}

func getBootPerformanceData(ctx context.Context, outDir string) (bootPerformanceInfo, error) {
	var bootPerf bootPerformanceInfo
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBootPerformance}
	err := croshealthd.RunAndParseJSONTelem(ctx, params, outDir, &bootPerf)

	return bootPerf, err
}

func validateBootPerformanceData(bootPerf *bootPerformanceInfo) error {
	if bootPerf.BootUpSeconds < 0.5 {
		return errors.New("Failed. It is impossible that boot_up_seconds is less than 0.5")
	}
	if bootPerf.BootUpTimestamp < 0.5 {
		return errors.New("Failed. It is impossible that boot_up_timestamp is less than 0.5")
	}
	if len(bootPerf.ShutdownReason) == 0 {
		return errors.New("Failed. shutdown_reason should not be empty string")
	}

	return nil
}

func ProbeBootPerformanceInfo(ctx context.Context, s *testing.State) {
	var bootPerf bootPerformanceInfo
	var err error
	if bootPerf, err = getBootPerformanceData(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to get boot performance telemetry info: ", err)
	}

	if err = validateBootPerformanceData(&bootPerf); err != nil {
		s.Fatal("Failed to validate boot performance data: ", err)
	}

	// Sleep 5 seconds, fetch data again. "boot_up_timestamp" should be the same.
	if err = testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	var bootPerfNew bootPerformanceInfo
	if bootPerfNew, err = getBootPerformanceData(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed to get boot performance telemetry info: ", err)
	}

	if err = validateBootPerformanceData(&bootPerfNew); err != nil {
		s.Fatal("Failed to validate boot performance data: ", err)
	}

	if math.Abs(bootPerf.BootUpTimestamp-bootPerfNew.BootUpTimestamp) > 0.1 {
		s.Errorf("Failed as difference between boot_up_timestamp (%v) and new boot_up_timestamp (%v) is greater than 0.1", bootPerf.BootUpTimestamp, bootPerfNew.BootUpTimestamp)
	}
}
