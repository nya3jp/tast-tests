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
		Func:     ProbeBatteryMetrics,
		Desc:     "Check that we can probe cros_healthd for battery metrics",
		Contacts: []string{"cros-tdm-tpe-eng@google.com"},
		// TODO(b/209014812): Test is unstable
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Fixture:      "crosHealthdRunning",
	})
}

func readBatteryStringProperty(path string) string {
	v, err := utils.ReadStringFile(path)
	if err != nil {
		return ""
	}
	return v
}

func readBatteryIntegerProperty(path string) int64 {
	raw := readBatteryStringProperty(path)
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func validateBatteryData(ctx context.Context, battery batteryInfo) error {
	sysfsPath, err := power.SysfsBatteryPath(ctx)
	if err != nil {
		return err
	}

	modelName := readBatteryStringProperty(sysfsPath + "/model_name")
	if battery.ModelName != modelName {
		return errors.Errorf("unexpected value for model_name: got %v; want %v", battery.ModelName, modelName)
	}

	serialNumber := readBatteryStringProperty(sysfsPath + "/serial_number")
	if battery.SerialNumber != serialNumber {
		return errors.Errorf("unexpected value for serial_number: got %v; want %v", battery.SerialNumber, serialNumber)
	}

	status := readBatteryStringProperty(sysfsPath + "/status")
	if battery.Status != status {
		return errors.Errorf("unexpected value for status: got %v; want %v", battery.Status, status)
	}

	technology := readBatteryStringProperty(sysfsPath + "/technology")
	if battery.Technology != technology {
		return errors.Errorf("unexpected value for technology: got %v; want %v", battery.Technology, technology)
	}

	vendor := readBatteryStringProperty(sysfsPath + "/manufacturer")
	if battery.Vendor != vendor {
		return errors.Errorf("unexpected value for vendor: got %v; want %v", battery.Vendor, vendor)
	}

	cycleCount := readBatteryStringProperty(sysfsPath + "/cycle_count")
	if battery.CycleCount != cycleCount {
		return errors.Errorf("unexpected value for cycle_count: got %v; want %v", battery.CycleCount, cycleCount)
	}

	chargeFull := float64(readBatteryIntegerProperty(sysfsPath+"/charge_full")) / 1e6
	if !utils.AlmostEqual(battery.ChargeFull, chargeFull) {
		return errors.Errorf("unexpected value for charge_full: got %v; want %v", battery.ChargeFull, chargeFull)
	}

	chargeFullDesign := float64(readBatteryIntegerProperty(sysfsPath+"/charge_full_design")) / 1e6
	if !utils.AlmostEqual(battery.ChargeFullDesign, chargeFullDesign) {
		return errors.Errorf("unexpected value for charge_full_design: got %v; want %v", battery.ChargeFullDesign, chargeFullDesign)
	}

	chargeNow := float64(readBatteryIntegerProperty(sysfsPath+"/charge_now")) / 1e6
	if !utils.AlmostEqual(battery.ChargeNow, chargeNow) {
		return errors.Errorf("unexpected value for charge_now: got %v; want %v", battery.ChargeNow, chargeNow)
	}

	currentNow := float64(readBatteryIntegerProperty(sysfsPath+"/current_now")) / 1e6
	if !utils.AlmostEqual(battery.CurrentNow, currentNow) {
		return errors.Errorf("unexpected value for current_now: got %v; want %v", battery.CurrentNow, currentNow)
	}

	voltageMinDesign := float64(readBatteryIntegerProperty(sysfsPath+"/voltage_min_design")) / 1e6
	if !utils.AlmostEqual(battery.VoltageMinDesign, voltageMinDesign) {
		return errors.Errorf("unexpected value for voltage_min_design: got %v; want %v", battery.VoltageMinDesign, voltageMinDesign)
	}

	voltageNow := float64(readBatteryIntegerProperty(sysfsPath+"/voltage_now")) / 1e6
	if !utils.AlmostEqual(battery.VoltageNow, voltageNow) {
		return errors.Errorf("unexpected value for voltage_now: got %v; want %v", battery.VoltageNow, voltageNow)
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

	if err := validateBatteryData(ctx, battery); err != nil {
		s.Fatalf("Failed to validate battery data, err [%v]", err)
	}
}
