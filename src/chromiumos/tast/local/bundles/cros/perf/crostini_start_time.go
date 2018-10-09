// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartTime,
		Desc:         "Performance tests of Termina VM startup and container startup",
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func measureOnce(ctx context.Context, s *testing.State, user string) time.Duration {

	start_time := time.Now()

	s.Log("Creating default container")

	cont, err := vm.CreateDefaultContainer(ctx, user, vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	elapsed_time := time.Since(start_time)
	s.Log("Elapsed time: ", elapsed_time)

	return elapsed_time
}

func CrostiniStartTime(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostiniSetting(ctx, tconn); err != nil {
		s.Fatal("Enable Crostini preference setting failed: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	duration := measureOnce(ctx, s, cr.User())
	start_time_in_secs := float64(duration) / float64(time.Second)

	metric := perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "total",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false}
	value := &perf.Values{}
	value.Set(metric, start_time_in_secs)
	value.Save(s.OutDir())
}
