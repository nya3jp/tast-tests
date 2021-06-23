// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type backlightInfo struct {
	Brightness    int    `json:"brightness"`
	MaxBrightness int    `json:"max_brightness"`
	Path          string `json:"path"`
}

type backlightResult struct {
	Backlights []backlightInfo `json:"backlights"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBacklightInfo,
		Desc: "Checks that cros_healthd can fetch backlight info",
		Contacts: []string{
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		// TODO(http://b/182185718): One kip device does not report having a backlight
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kip")),
		Fixture:      "crosHealthdRunning",
	})
}

func validateBacklightData(result backlightResult) error {
	for _, backlight := range result.Backlights {
		if backlight.Path == "" {
			return errors.New("failed, empty path")
		}
		if backlight.MaxBrightness < 0 {
			return errors.Errorf("invalid max_brightness value: %v", backlight.MaxBrightness)
		}
		if backlight.Brightness < 0 {
			return errors.Errorf("invalid brightness value: %v", backlight.Brightness)
		}
		if backlight.Brightness > backlight.MaxBrightness {
			return errors.Errorf("brightness: %v greater than max_brightness: %v", backlight.Brightness, backlight.MaxBrightness)
		}
	}

	return nil
}

func ProbeBacklightInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBacklight}
	rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get backlight telemetry info: ", err)
	}

	hasBacklight, err := crosconfig.Get(ctx, "/hardware-properties", "has-backlight")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get has-backlight property: ", err)
	}

	if err == nil && hasBacklight == "false" {
		// If there is no backlight, there is no output to verify.
		return
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.DisallowUnknownFields()

	var result backlightResult
	if err := dec.Decode(&result); err != nil {
		s.Fatalf("Failed to decode backlight data [%q], err [%v]", rawData, err)
	}

	if err := validateBacklightData(result); err != nil {
		s.Fatalf("Failed to validate backlight data, err [%v]", err)
	}
}
