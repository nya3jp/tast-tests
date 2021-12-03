// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type batteryInfo struct {
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
	CycleCount       jsontypes.Int64   `json:"cycle_count"`
	Temperature      *jsontypes.Uint64 `json:"temperature"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBatteryMetrics,
		Desc: "Check that we can probe cros_healthd for battery metrics",
		Contacts: []string{
			"pmoy@google.com",
			"khegde@google.com",
			"cros-tdm@google.com",
		},
		// TODO(b/209014812): Test is unstable
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func validateBatteryData(ctx context.Context, battery batteryInfo) error {
	if battery.ModelName == "" {
		return errors.New("Missing model_name")
	}
	if battery.SerialNumber == "" {
		return errors.New("Missing serial_number")
	}
	if battery.Status == "" {
		return errors.New("Missing status")
	}
	if battery.Technology == "" {
		return errors.New("Missing technology")
	}
	if battery.Vendor == "" {
		return errors.New("Missing vendor")
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
	psuType, err := crosconfig.Get(ctx, "/hardware-properties", "psu-type")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get psu-type property: ", err)
	}

	// If psu-type is not set to "AC_only", assume there is a battery.
	if err == nil && psuType == "AC_only" {
		// If there is no battery, there is no output to verify.
		return
	}

	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBattery}
	var battery batteryInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &battery); err != nil {
		s.Fatal("Failed to get battery telemetry info: ", err)
	}

	if err := validateBatteryData(ctx, battery); err != nil {
		s.Fatalf("Failed to validate battery data, err [%v]", err)
	}
}
