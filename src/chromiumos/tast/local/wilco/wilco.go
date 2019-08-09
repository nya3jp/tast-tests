// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wilco provides test utils for tests of the Wilco platform
package wilco

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/power"
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	// There is an upstart job that continually keeps the EC RTC in sync with
	// local time. We need to disable it during the test.
	upstartJobName = "wilco_sync_ec_rtc"
	// Timeout for running various commands.
	upstartTimeout = 3 * time.Second
)

var wilcoECRTC = rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true}

// WriteECRTC sets the RTC on the Wilco EC to a sepcified time.
// It fails the test if there are any problems.
func WriteECRTC(ctx context.Context, s *testing.State, t time.Time) {
	if err := wilcoECRTC.Write(ctx, t); err != nil {
		s.Fatalf("Failed to set EC RTC to %v: %v", t, err)
	}
}

// ReadECRTC reads the time from the RTC on the Wilco EC.
// It fails the test if there are any problems.
func ReadECRTC(ctx context.Context, s *testing.State) time.Time {
	t, err := wilcoECRTC.Read(ctx)
	if err != nil {
		s.Fatal("Failed to read EC RTC: ", err)
	}
	return t
}

// GetPowerStatus retrieves the present state of the charger and battery.
// It fails the test if there are any problems.
func GetPowerStatus(ctx context.Context, s *testing.State) *power.Status {
	status, err := power.GetStatus(ctx)
	if err != nil {
		s.Fatal("Failed to get power status: ", err)
	}
	return status
}

// StopSyncRTCJob stops the upstart job that keeps the EC RTC in sync with local time.
// It fails the test if there are any problems.
func StopSyncRTCJob(ctx context.Context, s *testing.State) {
	ctx, cancel := context.WithTimeout(ctx, upstartTimeout)
	defer cancel()
	if err := upstart.StopJob(ctx, upstartJobName); err != nil {
		s.Fatal("Failed to stop sync RTC upstart job: ", err)
	}
}

// StartSyncRTCJob starts the upstart job that keeps the EC RTC in sync with local time.
// It fails the test if there are any problems.
func StartSyncRTCJob(ctx context.Context, s *testing.State) {
	ctx, cancel := context.WithTimeout(ctx, upstartTimeout)
	defer cancel()
	if err := upstart.StartJob(ctx, upstartJobName); err != nil {
		s.Fatal("Failed to start sync RTC upstart job: ", err)
	}
}

// ReadFileStrict returns the data read from path.
// It fails the test if there are any problems.
func ReadFileStrict(s *testing.State, path string) string {
	res, err := ioutil.ReadFile(path)
	if err != nil {
		s.Fatalf("Failed to read from %s: %v", path, err)
	}
	return string(res)
}

// WriteFileStrict writes data to path, failing the test if there are any problems
// or if the file doesn't exist.
func WriteFileStrict(s *testing.State, path, data string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		s.Fatal("File does not exist: ", err)
	}
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		s.Fatalf("Failed to write %q to %s: %v", data, path, err)
	}
}
