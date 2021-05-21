// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"math"
	"reflect"
	"strconv"
	"time"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

const (
	powerBootUpSeconds   = "boot_up_seconds"
	powerBootUpTimestamp = "boot_up_timestamp"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbePowerInfo,
		Desc: "Check that we can probe cros_healthd for power info",
		Contacts: []string{
			"kerker@google.com",
			"cros-tdm@google.com",
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func getData(ctx context.Context, s *testing.State) map[string]string {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryPower}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get power telemetry info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of records: got %d; want 2", len(records))
	}

	// Verify the headers are correct.
	want := []string{powerBootUpSeconds, powerBootUpTimestamp}
	got := records[0]
	if !reflect.DeepEqual(want, got) {
		s.Fatalf("Incorrect headers: got %v; want %v", got, want)
	}

	// Verify the amount of values is correct.
	vals := records[1]
	if len(vals) != len(want) {
		s.Fatalf("Wrong number of values: got %d; want %d", len(vals), len(want))
	}

	// Using a map, then we don't need to take care of the index change in future.
	contentsMap := make(map[string]string)
	for i, elem := range want {
		contentsMap[elem] = vals[i]
	}

	return contentsMap
}

func ProbePowerInfo(ctx context.Context, s *testing.State) {
	contentsMap := getData(ctx, s)

	// Check "boot_up_seconds" and "boot_up_timestamp" is float.
	bootUpSeconds, err := strconv.ParseFloat(contentsMap[powerBootUpSeconds], 64)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to float: %v", contentsMap[powerBootUpSeconds], powerBootUpSeconds, err)
	}
	if bootUpSeconds < 0.5 {
		s.Errorf("Failed. It is impossible that %s is less than 0.5", powerBootUpSeconds)
	}

	bootUpTimestamp, err := strconv.ParseFloat(contentsMap[powerBootUpTimestamp], 64)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to float: %v", contentsMap[powerBootUpTimestamp], powerBootUpTimestamp, err)
	}
	if bootUpTimestamp < 0.5 {
		s.Errorf("Failed. It is impossible that %s is less than 0.5", powerBootUpTimestamp)
	}

	// Sleep 5 seconds, fetch data again. "boot_up_timestamp" should be the same.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Error("Failed to sleep: ", err)
	}

	contentsMapNew := getData(ctx, s)
	bootUpTimestampNew, err := strconv.ParseFloat(contentsMapNew[powerBootUpTimestamp], 64)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to float: %v", contentsMapNew[powerBootUpTimestamp], powerBootUpTimestamp, err)
	}
	if math.Abs(bootUpTimestamp-bootUpTimestampNew) > 0.1 {
		s.Errorf("Failed. bootUpTimestamp: %v, bootUpTimestampNew: %v", bootUpTimestamp, bootUpTimestampNew)
	}
}
