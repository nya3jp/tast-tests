// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelmeter provides a mechanism for collecting kernel-related
// measurements in parallel with the execution of a test.
package kernelmeter

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

// KernelMeter collects kernel performance statistics.
type KernelMeter struct {
	hasStarted bool
	isRunning  bool
	isReady    bool
	stop       chan struct{}
	stopped    chan struct{}
	ready      chan struct{}
	pfm        *pageFaultMeter
}

// pageFaultMeter collects page fault statistics.
type pageFaultMeter struct {
	startTime     time.Time
	startCount    int64
	maxStartTime  time.Time
	maxStartCount int64
	maxRate       float64
	finalTime     time.Time
	finalCount    int64
}

const samplePeriod = 1 * time.Second

// New creates a KernelMeter.
func New() *KernelMeter {
	kernelmeter := &KernelMeter{
		pfm: &pageFaultMeter{},
	}
	return kernelmeter
}

// initialize initializes the page fault meter.
func (pfm *pageFaultMeter) initialize() {
	pfm.startTime = time.Now()
	pfm.startCount = totalFaults()
	pfm.maxStartTime = pfm.startTime
	pfm.maxStartCount = pfm.startCount
	pfm.maxRate = 0.0
}

// totalFaults returns the total number of major page faults since boot.
// Panics if error occurs, since we expect the kernel to function properly.
func totalFaults() int64 {
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		panic("Cannot read /proc/vmstat")
	}
	chars := string(bytes)
	var value string
	for _, line := range strings.Split(chars, "\n") {
		if strings.HasPrefix(line, "pgmajfault ") {
			value = strings.Split(line, " ")[1]
			break
		}
	}
	if len(value) == 0 {
		panic("Cannot find pgmajfault in /proc/vmstat")
	}
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("Cannot parse pgmajfault value: %q", value))
	}
	return count
}

// Start starts the kernel meter, which samples the page fault rate
// periodically according to samplePeriod and tracks its max
// value.
func (km *KernelMeter) Start(ctx context.Context) {
	if km.hasStarted {
		panic("Kernel meter may only be started once")
	}
	km.isRunning = true
	km.stop = make(chan struct{})
	km.stopped = make(chan struct{})
	km.ready = make(chan struct{})
	km.hasStarted = true
	km.pfm.initialize()
	go func() {
		testing.ContextLog(ctx, "Kernel meter thread has started")
		defer func() {
			km.isRunning = false
			close(km.stopped)
			testing.ContextLog(ctx, "Kernel meter thread has stopped")
		}()
		for {
			select {
			case <-time.After(samplePeriod):
			case <-km.stop:
				return
			case <-ctx.Done():
				return
			}
			count := totalFaults()
			now := time.Now()
			// Use milliseconds resolution for rate computation
			interval := float64(now.Sub(km.pfm.startTime)/time.Millisecond) / 1000.0
			if interval > 0 {
				rate := float64(count-km.pfm.startCount) / interval
				if rate > km.pfm.maxRate {
					km.pfm.maxRate = rate
				}
			}
			km.pfm.maxStartTime = now
			km.pfm.maxStartCount = count
			if !km.isReady {
				km.isReady = true
				close(km.ready)
			}
		}
	}()
}

// Stop stops the kernel meter sampling thread.
func (km *KernelMeter) Stop() {
	if !km.isRunning {
		panic("Stopping kernel meter when it is not running")
	}
	close(km.stop)
	<-km.stopped
	km.pfm.finalTime = time.Now()
	km.pfm.finalCount = totalFaults()
}

// PageFaultStats returns the total number of page faults and the average and
// max page fault rate.  It will panic if the kernel meter has not run at all,
// or if it has not been stopped first.  It may block for a |samplePeriod|
// duration.
func (km *KernelMeter) PageFaultStats() (int64, float64, float64) {
	if !km.hasStarted {
		panic("Kernel meter never ran")
	}
	<-km.ready
	if km.isRunning {
		panic("Kernel meter must be stopped before collecting stats")
	}
	// Use millisecond resolution.
	interval := float64(km.pfm.finalTime.Sub(km.pfm.startTime)/time.Millisecond) / 1000.0
	faultCount := km.pfm.finalCount - km.pfm.startCount
	averageRate := float64(faultCount) / interval
	maxRate := km.pfm.maxRate
	return faultCount, averageRate, maxRate
}
