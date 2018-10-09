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
		Func: CrostiniStartTime,
		Desc: "Performance tests of Termina VM startup and container startup",
		// TODO(cylee): Change "disabled" to "crosbolt" after crbug/894375 is resolved.
		Attr:         []string{"informational", "disabled"},
		Timeout:      10 * time.Minute,
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
	defer func() {
		if vmInstance != nil {
			vmInstance.Close(ctx)
		}
	}()

	s.Log("Creating default container")
	cont, err := vmInstance.NewContainer(ctx, vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}

	s.Log("Starting default container")
	if err := cont.StartAndWait(ctx); err != nil {
		s.Fatal("Failed to start default container:", err)
	}

	s.Log("Shutting down VM")
	if err := vmInstance.Close(ctx); err != nil {
		s.Fatal("Failed to close VM: ", err)
	} else {
		vmInstance = nil
	}

	// Measure crostini starting time for |sampleNum| times.
	const sampleNum = 3

	// TODO(cylee): Add util function in perf.go to facilitate unit conversion.
	var vmStartTimesSec, containerStartTimesSec, vmShutdownTimesSec, totalStartTimesSec []float64
	for i := 0; i < sampleNum; i++ {
		s.Log("Sample ", i+1)
		s.Log("Restarting VM")

		startTime := time.Now()
		vmInstance, err = concierge.StartTerminaVM(ctx)
		if err != nil {
			s.Fatal("Restarting VM failed: ", err)
		}
		defer func() {
			if vmInstance != nil {
				vmInstance.Close(ctx)
			}
		}()

		duration := time.Since(startTime)

		s.Log("Elapsed time to start VM: ", duration)
		vmStartTimesSec = append(vmStartTimesSec, duration.Seconds())

		s.Log("Restarting container")

		startTime = time.Now()
		if err := cont.StartAndWait(ctx); err != nil {
			s.Fatal("Failed to restart container: ", err)
		}
		duration = time.Since(startTime)

		s.Log("Elapsed time to start container ", duration)
		containerStartTimesSec = append(containerStartTimesSec, duration.Seconds())

		s.Log("Shutting down VM")

		startTime = time.Now()
		if err := vmInstance.Close(ctx); err != nil {
			s.Fatal("Failed to close VM ", err)
		} else {
			vmInstance = nil
		}
		duration = time.Since(startTime)

		s.Log("Elapsed time to shut down VM ", duration)
		vmShutdownTimesSec = append(vmShutdownTimesSec, duration.Seconds())
	}
	for i := 0; i < sampleNum; i++ {
		totalStartTimesSec = append(totalStartTimesSec,
			vmStartTimesSec[i]+containerStartTimesSec[i])
	}

	value := &perf.Values{}
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "vm_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, vmStartTimesSec...)
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "container_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, containerStartTimesSec...)
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "vm_shutdown_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, vmShutdownTimesSec...)
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "total_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, totalStartTimesSec...)
	value.Save(s.OutDir())
}
