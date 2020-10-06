// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// IsDualQual checks if the test shall be run as dual-namespace AVL.
func IsDualQual(ctx context.Context, s *testing.State) bool {
	if val, ok := s.Var("storage.slcQual"); ok {
		dual, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Cannot parse argumet 'storage.QuickStress.slcQual' of type bool: ", err)
		}
		return dual
	}
	return false
}

// GetSlcDevice returns an Slc device path for dual-namespace AVL.
func GetSlcDevice(ctx context.Context, s *testing.State) string {
	info, err := ReadDiskInfo(ctx)
	if err != nil {
		s.Fatal("Failed reading disk info: ", err)
	}
	slc, err := info.SlcDevice()
	if slc == nil {
		s.Fatal("Dual qual is specified but SLC device is not present: ", err)
	}
	return "/dev/" + slc.Name
}

// RunFioStressForBootDev runs an fio job for the boot device.
// If fio returns an error, this function will fail the Tast test.
func RunFioStressForBootDev(ctx context.Context, s *testing.State, testConfig TestConfig) error {
	config := testConfig.WithPath(BootDeviceFioPath).WithJobFile(s.DataPath(testConfig.Job))

	if err := RunFioStress(ctx, config); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		s.Fatal("FIO stress failed: ", err)
	}
	return nil
}

// RunFioStressForSlcDev runs an fio job if slc namespace is present, otherwise
// returns immediately
// If fio returns an error, this function will fail the Tast test.
func RunFioStressForSlcDev(ctx context.Context, s *testing.State, testConfig TestConfig) error {
	if !IsDualQual(ctx, s) {
		return nil
	}
	config := testConfig.WithPath(GetSlcDevice(ctx, s)).WithJobFile(s.DataPath(testConfig.Job))

	if err := RunFioStress(ctx, config); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		s.Fatal("FIO stress failed: ", err)
	}
	return nil
}

// RunTasksInParallel runs stress tasks in parallel.
// Returns true if one or more tasks timed out.
func RunTasksInParallel(ctx context.Context, timeout time.Duration, tasks []func(ctx context.Context)) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	testing.ContextLog(ctx, "Starting parallel tasks at: ", time.Now())

	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(taskToRun func(ctx context.Context)) {
			taskToRun(ctx)
			wg.Done()
		}(task)
	}
	wg.Wait()
	testing.ContextLog(ctx, "Finished parallel tasks at: ", time.Now())

	return false
}

// WriteTestStatusFile writes test status JSON file to test's output folder.
// Status file contains start/end times and final test status (passed/failed).
func WriteTestStatusFile(ctx context.Context, s *testing.State, passed bool, startTimestamp time.Time) error {
	statusFileStruct := struct {
		Started  string `json:"started"`
		Finished string `json:"finished"`
		Passed   bool   `json:"passed"`
	}{
		Started:  startTimestamp.Format(time.RFC3339),
		Finished: time.Now().Format(time.RFC3339),
		Passed:   passed,
	}

	file, err := json.MarshalIndent(statusFileStruct, "", " ")
	if err != nil {
		return errors.Wrap(err, "failed marshalling test status to JSON")
	}
	filename := filepath.Join(s.OutDir(), "status.json")
	if err := ioutil.WriteFile(filename, file, 0644); err != nil {
		return errors.Wrap(err, "failed saving test status to file")
	}
	return nil
}
