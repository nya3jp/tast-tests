// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type fanInfo struct {
	SpeedRpm jsontypes.Uint32 `json:"speed_rpm"`
}

type fanResult struct {
	Fans []fanInfo `json:"fans"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeFanInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that cros_healthd can fetch fan info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func getNumFans(ctx context.Context) (int, error) {
	// Check to see if a Google EC exists. If it does, use ectool to get the
	// number of fans that should be reported. Otherwise, return 0 if the device
	// does not have a Google EC.
	if _, err := os.Stat("/sys/class/chromeos/cros_ec"); os.IsNotExist(err) {
		return 0, nil
	}

	b, err := testexec.CommandContext(ctx, "ectool", "pwmgetnumfans").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to run ectool command")
	}

	numFans, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(string(b)), "Number of fans = ", ""))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get number of fans")
	}

	return numFans, nil
}

func validateFanData(result *fanResult, numFans int) error {
	if len(result.Fans) != numFans {
		return errors.Errorf("Incorrect number of fans: got %d; want %d", len(result.Fans), numFans)
	}

	return nil
}

func ProbeFanInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryFan}
	var result fanResult
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &result); err != nil {
		s.Fatal("Failed to get fan telemetry info: ", err)
	}

	// Get the number of fans reported by ectool to check the number of records.
	numFans, err := getNumFans(ctx)
	if err != nil {
		s.Fatal("Failed to get number of fans: ", err)
	}

	if err := validateFanData(&result, numFans); err != nil {
		s.Fatalf("Failed to validate fan data, err [%v]", err)
	}
}
