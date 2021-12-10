// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"math"
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

func checkBatteryStringProperty(sysfsPath, field, want string) error {
	// When there is an error, |utils.ReadStringFile| returns empty string,
	// we can use empty string for future comparison and let's ignore the error.
	// Besides, sometimes we may fail to read sysfs file in image, but cros_healtd tast test doesn't need to cover the case.
	// Because in cros_healthd, we retrieve battery info from powerd.
	got, _ := utils.ReadStringFile(sysfsPath + "/" + field)
	if got != want {
		return errors.Errorf("unexpected value for %v: got %v, want %v", field, got, want)
	}
	return nil
}

func checkBatteryFloatProperty(sysfsPath, field string, want float64) error {
	s, err := utils.ReadStringFile(sysfsPath + "/" + field)
	if err != nil {
		return err
	}

	micros, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}

	// Because the value of battery varies continuously, so we only check if it's roughly the same.
	if got := float64(micros) / 1e6; math.Abs(got-want) > 1e-1 {
		return errors.Errorf("unexpected value for %v: got %v, want %v", field, got, want)
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
		"status":        battery.Status,
		"technology":    battery.Technology,
	}

	for field, want := range batteryStringFields {
		if err := checkBatteryStringProperty(sysfsPath, field, want); err != nil {
			return err
		}
	}

	batteryFloatFields := map[string]float64{
		"charge_full":        battery.ChargeFull,
		"charge_full_design": battery.ChargeFullDesign,
		"charge_now":         battery.ChargeNow,
		"current_now":        battery.CurrentNow,
		"voltage_min_design": battery.VoltageMinDesign,
		"voltage_now":        battery.VoltageNow,
	}

	for field, want := range batteryFloatFields {
		if err := checkBatteryFloatProperty(sysfsPath, field, want); err != nil {
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
