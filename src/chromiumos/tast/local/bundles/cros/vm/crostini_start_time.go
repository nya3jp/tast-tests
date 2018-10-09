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
		Attr:         []string{"informational", "crosbolt"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
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
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating VM")
	concierge, err := vm.NewConcierge(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to start Concierge: ", err)
	}

	vmInstance, err := concierge.StartTerminaVM(ctx)
	if err != nil {
		s.Fatal("Failed to create VM: ", err)
	}

	s.Log("Creating default container")
	cont, err := vmInstance.NewContainer(ctx, vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}

	s.Log("Starting default container")
	if err := cont.StartAndWait(ctx); err != nil {
		s.Fatal("Failed to start default container", err)
	}

	s.Log("Shutting down VM")
	if err := vmInstance.Close(ctx); err != nil {
		s.Fatal("Failed to close VM: ", err)
	}

	// Measure crostini starting time for |sampleNum| times.
	const sampleNum = 3

	var vm_start_times, container_start_times, vm_shutdown_times, total_times []float64
	for i := 0; i < sampleNum; i++ {
		s.Log("Sample ", i+1)
		s.Log("Restarting VM")

		startTime := time.Now()
		vmInstance, err = concierge.StartTerminaVM(ctx)
		if err != nil {
			s.Fatal("Restarting VM failed: ", err)
		}
		duration := time.Since(startTime)

		s.Log("Elapsed time to start VM: ", duration)
		vm_start_times = append(vm_start_times, duration.Seconds())

		s.Log("Restarting container")

		startTime = time.Now()
		if err := cont.StartAndWait(ctx); err != nil {
			s.Fatal("Failed to restart container: ", err)
		}
		duration = time.Since(startTime)

		s.Log("Elapsed time to start container: ", duration)
		container_start_times = append(container_start_times, duration.Seconds())

		s.Log("Shutting down VM")

		startTime = time.Now()
		if err := vmInstance.Close(ctx); err != nil {
			s.Fatal("Failed to close VM: ", err)
		}
		duration = time.Since(startTime)

		s.Log("Elapsed time to shut down VM: ", duration)
		vm_shutdown_times = append(vm_shutdown_times, duration.Seconds())
	}
	for i := 0; i < sampleNum; i++ {
		total_times = append(total_times, vm_start_times[i]+container_start_times[i])
	}

	value := &perf.Values{}
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "vm_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, vm_start_times...)
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "container_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, container_start_times...)
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "vm_shutdown_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, vm_shutdown_times...)
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "total_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, total_times...)
	value.Save(s.OutDir())
}
