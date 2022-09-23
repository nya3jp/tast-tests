// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"math"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type sensorInfo struct {
	LidAngle *uint16            `json:"lid_angle"`
	Sensors  []sensorAttributes `json:"sensors"`
}

type sensorAttributes struct {
	Name     *string `json:"name"`
	DeviceID int32   `json:"device_id"`
	Type     string  `json:"type"`
	Location string  `json:"location"`
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

// rawLidAngle parses the output of ectool and gets the raw value of lid angle.
// The return string should be a number or "unreliable".
func rawLidAngle(ctx context.Context) (string, error) {
	// Check to see if a Google EC exists. If it does, use ectool to get the lid
	// angle that should be reported. Otherwise, return "" if the device does not
	// have a Google EC.
	if _, err := os.Stat("/sys/class/chromeos/cros_ec"); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	bStdout, bStderr, err := testexec.CommandContext(ctx, "ectool", "motionsense", "lid_angle").SeparatedOutput(testexec.DumpLogOnError)
	if err != nil {
		stderr := string(bStderr)
		if strings.Contains(stderr, "INVALID_COMMAND") || strings.Contains(stderr, "INVALID_PARAM") {
			// Some devices do not support lid_angle and return |INVALID_COMMAND| or
			// |INVALID_PARAM|. Check stderr and return "" in these cases.
			return "", nil
		}
		return "", errors.Wrap(err, "failed to run ectool command")
	}

	return strings.ReplaceAll(strings.TrimSpace(string(bStdout)), "Lid angle: ", ""), nil
}

func validateLidAngle(ctx context.Context, info *sensorInfo) error {
	lidAngleRaw, err := rawLidAngle(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get lid angle")
	}

	if lidAngleRaw == "" || lidAngleRaw == "unreliable" {
		if info.LidAngle != nil {
			return errors.New("there is no reliable LidAngle, but cros_healthd report it")
		}
	} else {
		lidAngle, err := strconv.ParseUint(lidAngleRaw, 10, 16)
		if err != nil {
			return err
		}
		if info.LidAngle == nil {
			return errors.Errorf("failed. LidAngle doesn't match: got nil; want %v", lidAngle)
		}
		// The value of lid angle comes from the value of accelerometers on lid and
		// base, which is dynamic without user interaction. We should have the lid
		// angle tolerance.
		const lidAngleTolerance = 1
		if math.Abs(float64(*info.LidAngle)-float64(lidAngle)) > lidAngleTolerance {
			return errors.Errorf("failed. LidAngle doesn't match and the difference is out of tolerance: got %v; want %v", *info.LidAngle, lidAngle)
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
