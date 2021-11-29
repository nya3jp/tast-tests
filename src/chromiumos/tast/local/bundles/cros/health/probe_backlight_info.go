// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

type backlightInfo struct {
	Brightness    jsontypes.Uint32 `json:"brightness"`
	MaxBrightness jsontypes.Uint32 `json:"max_brightness"`
	Path          string           `json:"path"`
}

type backlightResult struct {
	Backlights []backlightInfo `json:"backlights"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBacklightInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cros_healthd can fetch backlight info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func validateBacklightData(result *backlightResult) error {
	for _, backlight := range result.Backlights {
		if backlight.Path == "" {
			return errors.New("failed, empty path")
		}
		if backlight.Brightness > backlight.MaxBrightness {
			return errors.Errorf("brightness: %v greater than max_brightness: %v", backlight.Brightness, backlight.MaxBrightness)
		}
	}

	return nil
}

func ProbeBacklightInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBacklight}
	var result backlightResult
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &result); err != nil {
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

	if err := validateBacklightData(&result); err != nil {
		s.Fatalf("Failed to validate backlight data, err [%v]", err)
	}
}
