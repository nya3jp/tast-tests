// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelmeter provides a mechanism for collecting kernel-related
// measurements in parallel with the execution of a test.
package kernelmeter

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Meter collects kernel performance statistics.
type Meter struct {
	isClosed bool            // true after the meter has been closed
	stop     chan struct{}   // closed (by client) to request stop
	stopped  chan struct{}   // closed by collection goroutine when it exits
	pfm      *pageFaultMeter // page-fault tracking data
}

// pageFaultMeter collects page fault statistics.
type pageFaultMeter struct {
	startTime        time.Time  // time of collection start
	startCount       int64      // page fault counter at start
	sampleStartTime  time.Time  // start time of sample period for sample rate
	sampleStartCount int64      // page fault counter at start of sample period
	maxRate          float64    // max seen page fault rate
	mutex            sync.Mutex // for safe access of all variables
}

// PageFaultData is used to return page fault statistics.
type PageFaultData struct {
	Count       int64   // how many page faults have occurred
	AverageRate float64 // average rate for the duration of the sampling
	MaxRate     float64 // max rate seen in a short interval during the sampling
}

const samplePeriod = 1 * time.Second // length of sample period for max rate calculation

// New creates a Meter and starts the sampling goroutine.
func New(ctx context.Context) *Meter {
	pfm := &pageFaultMeter{}
	pfm.reset()
	m := &Meter{
		pfm:     pfm,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go m.start(ctx)
	return m
}

// Close stops the sampling goroutine and releases other resources.
func (m *Meter) Close(ctx context.Context) {
	if m.isClosed {
		panic("Closing already closed kernelmeter")
	}
	// Send stop request to the goroutine.
	close(m.stop)
	// Wait for the goroutine to finish.
	select {
	case <-m.stopped:
	case <-ctx.Done():
	}
	m.isClosed = true
}

// Reset resets a Meter so that it is ready for a new set of measurements.
func (m *Meter) Reset() {
	m.pfm.reset()
}

// Reset initializes or resets a pageFaultMeter.
func (pfm *pageFaultMeter) reset() {
	now := time.Now()
	count := totalFaults()

	pfm.mutex.Lock()
	defer pfm.mutex.Unlock()

	pfm.startTime = now
	pfm.startCount = count
	pfm.sampleStartTime = now
	pfm.sampleStartCount = count
	pfm.maxRate = 0.0
}

// sampleMax records the maximum page fault rate seen after a reset.
func (pfm *pageFaultMeter) sampleMax() {
	count := totalFaults()
	now := time.Now()

	pfm.mutex.Lock()
	defer pfm.mutex.Unlock()

	interval := now.Sub(pfm.sampleStartTime).Seconds()
	if interval > 0 {
		rate := float64(count-pfm.sampleStartCount) / interval
		if rate > pfm.maxRate {
			pfm.maxRate = rate
		}
	}
	pfm.sampleStartTime = now
	pfm.sampleStartCount = count
}

// stats returns the page fault stats since the last reset.
func (pfm *pageFaultMeter) stats() (*PageFaultData, error) {
	pfm.mutex.Lock()
	defer pfm.mutex.Unlock()

	count := totalFaults() - pfm.startCount
	interval := time.Now().Sub(pfm.startTime).Seconds()
	if interval == 0.0 {
		return nil, errors.New("calling PageFaultStats too soon")
	}
	return &PageFaultData{
		Count:       count,
		AverageRate: float64(count) / interval,
		MaxRate:     pfm.maxRate,
	}, nil
}

// totalFaults returns the total number of major page faults since boot.
// Panics if any error occurs, since we expect the kernel to function properly.
func totalFaults() int64 {
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		panic(fmt.Sprint("Cannot read /proc/vmstat: ", err))
	}
	var value string
	for _, line := range strings.Split(string(bytes), "\n") {
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
		panic(fmt.Sprintf("Cannot parse pgmajfault value %q: %v", value, err))
	}
	return count
}

// start starts the kernel meter, which samples the page fault rate
// periodically according to samplePeriod to track its max value.
func (m *Meter) start(ctx context.Context) {
	testing.ContextLog(ctx, "Kernel meter goroutine has started")
	defer func() {
		close(m.stopped)
		testing.ContextLog(ctx, "Kernel meter goroutine has stopped")
	}()
	for {
		select {
		case <-time.After(samplePeriod):
		case <-m.stop:
			return
		case <-ctx.Done():
			return
		}

		m.pfm.sampleMax()
	}
}

// PageFaultStats returns the total number of page faults, and the average and
// max page fault rate.
func (m *Meter) PageFaultStats() (*PageFaultData, error) {
	return m.pfm.stats()
}

// Meminfo returns the name/value pairs from /proc/meminfo as a map.  Panic on
// errors, since the calling context doesn't matter, and in any case
// /proc/meminfo is highly stable.
func Meminfo() map[string]int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		panic("Cannot open /proc/meminfo")
	}
	m := make(map[string]int)
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		fields := strings.Fields(line)
		if len(fields) != 3 {
			panic(fmt.Sprintf("Unexpected /proc/meminfo entry: %s", line))
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			panic(fmt.Sprintf("Bad value in /proc/meminfo entry: %s", line))
		}
		// Remove the colon in the name field.
		name := fields[0]
		if len(name) == 0 || name[len(name)-1] != ':' {
			panic(fmt.Sprintf("Missing colon in /proc/meminfo entry: %s", line))
		}
		m[fields[0][:len(fields[0])-1]] = n
	}
	return m
}
