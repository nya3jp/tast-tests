// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type sensorInfo struct {
	LidAngle *uint16 `json:"lid_angle"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeSensorInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cros_healthd can fetch sensor info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func getLidAngleRaw(ctx context.Context) (string, error) {
	// Check to see if a Google EC exists. If it does, use ectool to get the
	// number of fans that should be reported. Otherwise, return 0 if the device
	// does not have a Google EC.
	if _, err := os.Stat("/sys/class/chromeos/cros_ec"); os.IsNotExist(err) {
		return "", nil
	}

	b, err := testexec.CommandContext(ctx, "ectool", "motionsense", "lid_angle").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run ectool command")
	}

	return strings.ReplaceAll(strings.TrimSpace(string(b)), "Lid angle: ", ""), nil
}

func validateLidAngle(ctx context.Context, info *sensorInfo) error {
	lidAngleRaw, err := getLidAngleRaw(ctx)
	if err != nil {
		errors.Wrap(err, "failed to get lid angle")
	}

	if lidAngleRaw == "" || lidAngleRaw == "unreliable" {
		if info.LidAngle != nil {
			return errors.New("there is no reliable LidAngle, but cros_healthd report it")
		}
	} else {
		if lidAngle, err := strconv.ParseUint(lidAngleRaw, 10, 16); err != nil {
			return err
		} else if info.LidAngle == nil {
			errors.Errorf("failed. LidAngle doesn't match: got nil; want %v", lidAngle)
		} else if *info.LidAngle != uint16(lidAngle) {
			return err
		}
	}

	return nil
}

func ProbeSensorInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySensor}
	var info sensorInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get sensor telemetry info: ", err)
	}

	if err := validateLidAngle(ctx, &info); err != nil {
		s.Fatalf("Failed to validate sensor data, err [%v]", err)
	}
}
