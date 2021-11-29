// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/health/utils"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type batteryInfo struct {
	CycleCount       string            `json:"cycle_count"`
	ModelName        string            `json:"model_name"`
	SerialNumber     string            `json:"serial_number"`
	Status           string            `json:"status"`
	Technology       string            `json:"technology"`
	Vendor           string            `json:"vendor"`
	ManufactureDate  *string           `json:"manufacture_date"`
	ChargeFull       float64           `json:"charge_full"`
	ChargeFullDesign float64           `json:"charge_full_design"`
	ChargeNow        float64           `json:"charge_now"`
	CurrentNow       float64           `json:"current_now"`
	VoltageMinDesign float64           `json:"voltage_min_design"`
	VoltageNow       float64           `json:"voltage_now"`
	Temperature      *jsontypes.Uint64 `json:"temperature"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBatteryMetrics,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for battery metrics",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		// TODO(b/209014812): Test is unstable
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Fixture:      "crosHealthdRunning",
	})
}

func checkBatteryStringProperty(sysfsPath, field, got string) error {
	// When there is an error to read answer from sysfs file (e.g. IO error, file missing),
	// it means that powerd will also report empty value to cros_healthd.
	// So cros_healthd reports empty string in this case.
	//
	// What we want to verify in this test is make sure that we align with the powerd behavior.
	// This kind of error is out of this test's scope.
	//
	// Our goal:
	// 1. Make sure there is no crash when fetching data.
	// 2. Make sure that cros_healthd can have same output with powerd.
	want, _ := utils.ReadStringFile(sysfsPath + "/" + field)
	if got != want {
		return errors.Errorf("unexpected value for %v: got %v, want %v", field, got, want)
	}
	return nil
}

func checkBatteryFloatProperty(sysfsPath, field string, got float64) error {
	s, err := utils.ReadStringFile(sysfsPath + "/" + field)
	if err != nil {
		return err
	}

	micros, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}

	// Because the value of battery varies continuously, so we only check if it's roughly the same.
	// Checked with hardware team, they recommended that we can check if it's within 5%.
	want := float64(micros) / 1e6
	maxWant := want * 1.05
	minWant := want * 0.95
	if got > maxWant || got < minWant {
		return errors.Errorf("unexpected value for %v: got %v, want [%v, %v]", field, got, minWant, maxWant)
	}
	return nil
}

func validateBatteryData(ctx context.Context, battery *batteryInfo) error {
	sysfsPath, err := power.SysfsBatteryPath(ctx)
	if err != nil {
		return err
	}

	batteryStringFields := map[string]string{
		"cycle_count":   battery.CycleCount,
		"manufacturer":  battery.Vendor,
		"model_name":    battery.ModelName,
		"serial_number": battery.SerialNumber,
		"technology":    battery.Technology,
	}

	for field, got := range batteryStringFields {
		if err := checkBatteryStringProperty(sysfsPath, field, got); err != nil {
			return err
		}
	}

	// Battery status changes from time to time, so we only check if the status string is expected or not.
	_, ok := power.MapStringToBatteryStatus(battery.Status)
	if !ok {
		return errors.Errorf("status %v is not expected", battery.Status)
	}

	batteryFloatFields := map[string]float64{
		"charge_full":        battery.ChargeFull,
		"charge_full_design": battery.ChargeFullDesign,
		"charge_now":         battery.ChargeNow,
		"voltage_min_design": battery.VoltageMinDesign,
		"voltage_now":        battery.VoltageNow,
		// Skip float64 fields:
		//
		// |current_now|
		// We can't test it, because the value varies quickly.
		// For example, cros_healthd get 0.639 but when we fetch the value from sysfs, it becomes 0.961.
	}

	for field, got := range batteryFloatFields {
		if err := checkBatteryFloatProperty(sysfsPath, field, got); err != nil {
			return err
		}
	}

	// Validate Smart Battery metrics.
	val, err := crosconfig.Get(ctx, "/cros-healthd/battery", "has-smart-battery-info")
	if err != nil && !crosconfig.IsNotFound(err) {
		return errors.Wrap(err, "failed to get has-smart-battery-info property")
	}

	hasSmartInfo := err == nil && val == "true"
	if hasSmartInfo {
		if battery.ManufactureDate == nil {
			return errors.New("Missing manufacture_date for smart battery")
		}
		if battery.Temperature == nil {
			return errors.New("Missing temperature for smart battery")
		}
	}

	return nil
}

func ProbeBatteryMetrics(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBattery}
	var battery batteryInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &battery); err != nil {
		s.Fatal("Failed to get battery telemetry info: ", err)
	}

	if err := validateBatteryData(ctx, &battery); err != nil {
		s.Fatal("Failed to validate battery data: ", err)
	}
}
