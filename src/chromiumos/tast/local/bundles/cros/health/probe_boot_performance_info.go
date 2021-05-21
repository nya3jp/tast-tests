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
	bootPerformanceBootUpSeconds   = "boot_up_seconds"
	bootPerformanceBootUpTimestamp = "boot_up_timestamp"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBootPerformanceInfo,
		Desc: "Check that we can probe cros_healthd for boot performance info",
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
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBootPerformance}
	records, err := croshealthd.RunAndParseTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get boot performance telemetry info: ", err)
	}

	if len(records) != 2 {
		s.Fatalf("Wrong number of records: got %d; want 2", len(records))
	}

	// Verify the headers are correct.
	want := []string{bootPerformanceBootUpSeconds, bootPerformanceBootUpTimestamp}
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

func ProbeBootPerformanceInfo(ctx context.Context, s *testing.State) {
	contentsMap := getData(ctx, s)

	// Check "boot_up_seconds" and "boot_up_timestamp" is float.
	bootUpSeconds, err := strconv.ParseFloat(contentsMap[bootPerformanceBootUpSeconds], 64)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to float: %v", contentsMap[bootPerformanceBootUpSeconds], bootPerformanceBootUpSeconds, err)
	} else if bootUpSeconds < 0.5 {
		s.Errorf("Failed. It is impossible that %s is less than 0.5", bootPerformanceBootUpSeconds)
	}

	bootUpTimestamp, err := strconv.ParseFloat(contentsMap[bootPerformanceBootUpTimestamp], 64)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to float: %v", contentsMap[bootPerformanceBootUpTimestamp], bootPerformanceBootUpTimestamp, err)
	} else if bootUpTimestamp < 0.5 {
		s.Errorf("Failed. It is impossible that %s is less than 0.5", bootPerformanceBootUpTimestamp)
	}

	// Sleep 5 seconds, fetch data again. "boot_up_timestamp" should be the same.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	contentsMapNew := getData(ctx, s)
	bootUpTimestampNew, err := strconv.ParseFloat(contentsMapNew[bootPerformanceBootUpTimestamp], 64)
	if err != nil {
		s.Errorf("Failed to convert %q (%s) to float: %v", contentsMapNew[bootPerformanceBootUpTimestamp], bootPerformanceBootUpTimestamp, err)
	} else if math.Abs(bootUpTimestamp-bootUpTimestampNew) > 0.1 {
		s.Errorf("Failed. bootUpTimestamp: %v, bootUpTimestampNew: %v", bootUpTimestamp, bootUpTimestampNew)
	}
}
