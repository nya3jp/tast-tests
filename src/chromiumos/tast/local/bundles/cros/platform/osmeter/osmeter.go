// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package osmeter

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/testing"
)

// OsMeter collects kernel performance statistics.
type OsMeter struct {
	ctx                         context.Context
	pageFaultMeterMutex         sync.Mutex
	pageFaultMeterStartTime     time.Time
	pageFaultMeterStartCount    int64
	pageFaultMeterMaxStartTime  time.Time
	pageFaultMeterMaxStartCount int64
	pageFaultMeterMaxRate       float64
}

const pageFaultMeterSamplePeriod = 1 * time.Second

// New creates an OsMeter with the given context.
func New(ctx context.Context) *OsMeter {
	osmeter := &OsMeter{
		ctx: ctx,
	}
	osmeter.PageFaultMeterReset()
	return osmeter
}

// getPageFaultCount returns the total number of major page faults since boot.
// If errors occur, they are logged and the function returns 0.
func (osmeter *OsMeter) getPageFaultCount() int64 {
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		testing.ContextLog(osmeter.ctx, "Cannot read /proc/vmstat")
		return 0.0
	}
	chars := string(bytes)
	lines := strings.Split(chars, "\n")
	var value string
	for i := range lines {
		if strings.HasPrefix(lines[i], "pgmajfault ") {
			value = strings.Split(lines[i], " ")[1]
		}
	}
	if len(value) == 0 {
		testing.ContextLog(osmeter.ctx, "Cannot find pgmajfault in /proc/vmstat")
		return 0.0
	}
	pageFaultCount, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		testing.ContextLog(osmeter.ctx, "Cannot parse pgmajfault value: ", value)
		return 0.0
	}
	return pageFaultCount
}

// PageFaultMeterReset resets the page fault meter.
func (osmeter *OsMeter) PageFaultMeterReset() {
	osmeter.pageFaultMeterMutex.Lock()
	osmeter.pageFaultMeterStartTime = time.Now()
	osmeter.pageFaultMeterStartCount = osmeter.getPageFaultCount()
	osmeter.pageFaultMeterMaxStartTime = osmeter.pageFaultMeterStartTime
	osmeter.pageFaultMeterMaxStartCount = osmeter.pageFaultMeterStartCount
	osmeter.pageFaultMeterMaxRate = 0.0
	osmeter.pageFaultMeterMutex.Unlock()
}

// PageFaultMeterRun runs the page fault meter, which samples the page fault rate
// periodically according to pageFaultMeterSamplePeriod and tracks its max value.
// It is meant to be run as a goroutine.
func (osmeter *OsMeter) PageFaultMeterRun() {
	for {
		select {
		case <-time.After(pageFaultMeterSamplePeriod):
		case <-osmeter.ctx.Done():
			return
		}
		count := osmeter.getPageFaultCount()
		now := time.Now()
		osmeter.pageFaultMeterMutex.Lock()
		interval := float64(now.Sub(osmeter.pageFaultMeterMaxStartTime) / time.Second)
		if interval > 0 {
			rate := float64(count-osmeter.pageFaultMeterMaxStartCount) / interval
			if rate > osmeter.pageFaultMeterMaxRate {
				osmeter.pageFaultMeterMaxRate = rate
			}
		}
		osmeter.pageFaultMeterMaxStartTime = now
		osmeter.pageFaultMeterMaxStartCount = count
		osmeter.pageFaultMeterMutex.Unlock()
	}
}

// PageFaultMeterGetStats returns the total number of page faults and the
// average and max page fault rate.  It may block for up to
// |pageFaultMeterSamplePeriod| if called too soon after a reset.
func (osmeter *OsMeter) PageFaultMeterGetStats(ctx context.Context) (faultCount int64, averageRate, maxRate float64) {
	now := time.Now()
	osmeter.pageFaultMeterMutex.Lock()
	readyTime := osmeter.pageFaultMeterStartTime.Add(pageFaultMeterSamplePeriod)
	if now.Before(readyTime) {
		osmeter.pageFaultMeterMutex.Unlock()
		testing.ContextLog(ctx, "Delaying premature call to PageFaultMeterGetStats")
		select {
		case <-time.After(readyTime.Sub(now)):
		case <-osmeter.ctx.Done():
		}
		now = time.Now()
	}
	// Convert to milliseconds to avoid coarse rounding.
	interval := float64(now.Sub(osmeter.pageFaultMeterStartTime)/time.Millisecond) / 1000.0
	totalCount := osmeter.getPageFaultCount()
	faultCount = totalCount - osmeter.pageFaultMeterStartCount
	averageRate = float64(faultCount) / interval
	maxRate = osmeter.pageFaultMeterMaxRate
	osmeter.pageFaultMeterMutex.Unlock()
	return
}
