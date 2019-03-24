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
		Contacts:     []string{"cylee@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func createNewContainer(ctx context.Context, vmInstance *vm.VM) (cont *vm.Container, elapsedTime time.Duration, err error) {
	cont, err = vm.DefaultContainer(ctx, vmInstance)
	if err != nil {
		return nil, 0, err
	}

	watcher, err := vm.NewContainerCreationWatcher(ctx, cont)
	if err != nil {
		return nil, 0, err
	}
	defer watcher.Close(ctx)

	startTime := time.Now()

	if err := cont.Create(ctx, vm.StagingImageServer); err != nil {
		return nil, 0, err
	}

	if err := watcher.WaitForDownload(ctx, -1); err != nil {
		return nil, 0, err
	}
	// Pause timer when downloading begins.
	elapsedTime = time.Since(startTime)
	testing.ContextLog(ctx, "Container downloading")

	if err := watcher.WaitForDownload(ctx, 100); err != nil {
		return nil, 0, err
	}
	testing.ContextLog(ctx, "Container downloaded")
	// Resume timer when download is finished.
	startTime = time.Now()

	if err := watcher.WaitForCreationComplete(ctx); err != nil {
		return nil, 0, err
	}
	elapsedTime += time.Since(startTime)

	return cont, elapsedTime, nil
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

	s.Log("Restarting Concierge")
	concierge, err := vm.NewConcierge(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to start Concierge: ", err)
	}

	type measurements struct {
		vmStart         time.Duration
		containerCreate time.Duration // set only when container needs to be created
		containerStart  time.Duration
		vmShutdown      time.Duration
	}

	vmInstance := vm.NewDefaultVM(concierge)
	var cont *vm.Container

	measure := func() *measurements {
		timing := &measurements{}

		s.Log("Starting VM")
		startTime := time.Now()
		err := vmInstance.Start(ctx)
		if err != nil {
			s.Fatal("Failed to start VM: ", err)
		}
		timing.vmStart = time.Since(startTime)
		s.Log("Elapsed time to start VM ", timing.vmStart.Round(time.Millisecond))

		// Create default container for the initial run.
		if cont == nil {
			s.Log("Creating default container")
			cont, timing.containerCreate, err = createNewContainer(ctx, vmInstance)
			if err != nil {
				s.Fatal("Failed to set up default container: ", err)
			}
			s.Log("Elapsed time to create container ", timing.containerCreate.Round(time.Millisecond))
		}

		s.Log("Starting default container")
		startTime = time.Now()
		if err := cont.StartAndWait(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start default container: ", err)
		}
		timing.containerStart = time.Since(startTime)
		s.Log("Elapsed time to start container ", timing.containerStart.Round(time.Millisecond))

		s.Log("Shutting down VM")
		startTime = time.Now()
		err = vmInstance.Stop(ctx)
		if err != nil {
			s.Fatal("Failed to close VM: ", err)
		}
		timing.vmShutdown = time.Since(startTime)
		s.Log("Elapsed time to shut down VM ", timing.vmShutdown.Round(time.Millisecond))

		return timing
	}

	value := perf.NewValues()

	// Initial setup measurement.
	timing := measure()
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "container_creation_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, timing.containerCreate.Seconds())
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "initial_vm_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, timing.vmStart.Seconds())
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "initial_container_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, timing.containerStart.Seconds())
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "initial_vm_shutdown_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, timing.vmShutdown.Seconds())
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "initial_total_start_time",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, (timing.vmStart + timing.containerCreate + timing.containerStart).Seconds())

	// Measure crostini starting time for |sampleNum| times.
	const sampleNum = 3

	for i := 0; i < sampleNum; i++ {
		s.Log("Sample ", i+1)
		timing := measure()

		value.Append(perf.Metric{
			Name:      "crostini_start_time",
			Variant:   "vm_start_time",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, timing.vmStart.Seconds())
		value.Append(perf.Metric{
			Name:      "crostini_start_time",
			Variant:   "container_start_time",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, timing.containerStart.Seconds())
		value.Append(perf.Metric{
			Name:      "crostini_start_time",
			Variant:   "vm_shutdown_time",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, timing.vmShutdown.Seconds())
		value.Append(perf.Metric{
			Name:      "crostini_start_time",
			Variant:   "total_start_time",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, (timing.vmStart + timing.containerStart).Seconds())
	}
	value.Save(s.OutDir())
}
