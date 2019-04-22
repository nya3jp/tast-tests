// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernelmeter provides a mechanism for collecting kernel-related
// measurements in parallel with the execution of a test.
//
// Several kernel quantities (e.g page faults, swaps) are exposed via sysfs or
// procfs in the form of counters.  We are generally interested in the absolute
// increments of these values over a period of time, and their rate of change.
// A kernelmeter.Meter instance keeps track of the initial values of the
// counters so that deltas can be computed.  It also calculates the peak rate
// over an interval.  Additionally, various methods are available for reading
// snapshots of other exported kernel quantities.
package kernelmeter

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
)

// Meter collects kernel performance statistics.
type Meter struct {
	isClosed bool          // true after the meter has been closed
	stop     chan struct{} // closed (by client) to request stop
	stopped  chan struct{} // closed by collection goroutine when it exits
	vmsm     *vmStatsMeter // tracks various memory manager counters
}

// vmField is an index into vmSample.fields.  Each vmstat of interest is
// assigned a fixed vmField.
type vmField int

// The /proc/vmstat fields to be collected.
const (
	pageFaultField vmField = iota
	swapInField
	swapOutField
	oomKillField
	vmFieldsLastField
)

const vmFieldsLength = int(vmFieldsLastField)

// vmSample contains a snapshot (time + values) from /proc/vmstat.
type vmSample struct {
	time   time.Time
	fields [vmFieldsLength]uint64
}

// vmFieldIndices maps the name of a vmstat field to a vmField, which is an
// index into a vmSample.fields vector.
var vmFieldIndices = map[string]vmField{
	"pgmajfault": pageFaultField,
	"pswpin":     swapInField,
	"pswpout":    swapOutField,
	"oom_kill":   oomKillField,
}

const (
	// Length of window for moving averages as a multiple of the sampling period.
	vmCountWindowLength = 10
	// Number of samples in circular buffer.  The window has samples at
	// both ends, so for instance a window of length 1 requires 2 samples.
	sampleBufferLength = vmCountWindowLength + 1
)

// vmStatsMeter collects vm counter statistics.
type vmStatsMeter struct {
	startSample vmSample                     // initial values at collection start
	samples     [sampleBufferLength]vmSample // circular buffer of recent samples
	sampleIndex int                          // index of most recent sample in buffer
	sampleCount int                          // count of valid samples in buffer (for startup)
	maxRates    [vmFieldsLength]float64      // max seen counter rates (delta per second)
	mutex       sync.Mutex                   // for safe access of all variables
}

// reset resets a vmStatsMeter.  Should be called immediately after
// acquireSample, so that the latest sample is up to date.  Note that this
// resets the start time and max rates seen, but does not modify the
// circular buffer used to compute the moving average.
func (v *vmStatsMeter) reset() {
	v.startSample = v.samples[v.sampleIndex]
	for i := range v.maxRates {
		v.maxRates[i] = 0.0
	}
}

// updateMaxRates updates the max rate of increase seen for each counter.
func (v *vmStatsMeter) updateMaxRates() {
	currentTime := v.samples[v.sampleIndex].time
	previousIndex := (v.sampleIndex - 1 + sampleBufferLength) % sampleBufferLength
	previousTime := v.samples[previousIndex].time
	for i := 0; i < vmFieldsLength; i++ {
		currentCount := v.samples[v.sampleIndex].fields[i]
		previousCount := v.samples[previousIndex].fields[i]
		rate := float64(currentCount-previousCount) / currentTime.Sub(previousTime).Seconds()
		if rate > v.maxRates[i] {
			v.maxRates[i] = rate
		}
	}
}

// acquireSample adds a new sample to the circular buffer, and tracks the
// number of valid entries in the buffer.
func (v *vmStatsMeter) acquireSample() {
	if v.sampleCount < sampleBufferLength {
		v.sampleCount++
	}
	v.sampleIndex = (v.sampleIndex + 1) % sampleBufferLength
	v.samples[v.sampleIndex].read()
}

// counterData produces a VMCounterData for field.
func (v *vmStatsMeter) counterData(field vmField) VMCounterData {
	current := v.samples[v.sampleIndex].fields[field]
	currentTime := v.samples[v.sampleIndex].time
	delta := current - v.startSample.fields[field]
	// Use the most recent and least recent samples in the circular buffer.
	old := (v.sampleIndex - (v.sampleCount - 1) + sampleBufferLength) % sampleBufferLength
	oldTime := v.samples[old].time
	recentDelta := current - v.samples[old].fields[field]
	return VMCounterData{
		Count:       delta,
		AverageRate: float64(delta) / currentTime.Sub(v.startSample.time).Seconds(),
		MaxRate:     v.maxRates[field],
		RecentRate:  float64(recentDelta) / currentTime.Sub(oldTime).Seconds(),
	}
}

// VMCounterData contains statistics for a memory manager event counter, such
// as the page fault counter (pgmajfault in /proc/vmstat).
type VMCounterData struct {
	// Count is the number of events since the last reset.
	Count uint64
	// AverageRate is the average rate (increase/second) for the duration
	// of the sampling.
	AverageRate float64
	// MaxRate is the maximum rate seen during the sampling
	// (increase/second over samplePeriod intervals).
	MaxRate float64
	// RecentRate is the average rate in the most recent window with size
	// vmCountWindowLength periods (or slightly more), or however many
	// periods are available since the most recent reset, including the
	// most recent sample.
	RecentRate float64
}

// VMStatsData contains statistics for various memory manager counters.
// The fields of VMStatsData must match the names and indices above.
type VMStatsData struct {
	// PageFault reports major page fault count and rates.
	PageFault VMCounterData
	// SwapIn reports swapin count and rates.
	SwapIn VMCounterData
	// SwapOut reports swapout count and rates.
	SwapOut VMCounterData
	// OOM reports out-of-memory kill count and rates.
	OOM VMCounterData
}

const samplePeriod = 1 * time.Second // length of sample period for max rate calculation

// New creates a Meter and starts the sampling goroutine.
func New(ctx context.Context) *Meter {
	vmsm := newVMStatsMeter()
	m := &Meter{
		vmsm:    vmsm,
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
	m.vmsm.reset()
}

// newVMStatsMeter returns a vmStatsMeter instance.
func newVMStatsMeter() *vmStatsMeter {
	v := &vmStatsMeter{}
	v.acquireSample()
	v.reset()
	return v
}

// stats returns the vm counter stats since the last reset.
func (v *vmStatsMeter) stats() (*VMStatsData, error) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	interval := time.Now().Sub(v.startSample.time)
	if interval.Seconds() == 0.0 {
		return nil, errors.New("calling VMCounterStats too soon")
	}
	v.acquireSample()
	return &VMStatsData{
		PageFault: v.counterData(pageFaultField),
		SwapIn:    v.counterData(swapInField),
		SwapOut:   v.counterData(swapOutField),
		OOM:       v.counterData(oomKillField),
	}, nil
}

// read stores the current time and current values of selected fields of
// /proc/vmstat into s.  Panics if any error occurs, since we expect the kernel
// to function properly.  The values of fields that are not found in
// /proc/vmstat are left unchanged.
func (s *vmSample) read() {
	s.time = time.Now()
	b, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		panic(fmt.Sprint("Cannot read /proc/vmstat: ", err))
	}
	seen := make(map[string]struct{})
	lines := strings.Split(string(b), "\n")
	for _, line := range lines[:len(lines)-1] {
		nameValue := strings.Split(line, " ")
		name := nameValue[0]
		value := nameValue[1]
		i, present := vmFieldIndices[name]
		if !present {
			continue
		}
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("Cannot parse %q value %q: %v", name, value, err))
		}
		s.fields[i] = uint64(count)
		if len(seen) == vmFieldsLength {
			break
		}
	}
}

// start starts the kernel meter, which periodically samples various memory
// manager quantities (such as page fault counts) and tracks the max values of
// their rate of change.
func (m *Meter) start(ctx context.Context) {
	defer func() {
		close(m.stopped)
	}()
	for {
		select {
		case <-time.After(samplePeriod):
		case <-m.stop:
			return
		case <-ctx.Done():
			return
		}
		m.vmsm.acquireSample()
		m.vmsm.updateMaxRates()
	}
}

// VMStats returns the total number of events, and the average and
// max rates, for various memory manager events.
func (m *Meter) VMStats() (*VMStatsData, error) {
	return m.vmsm.stats()
}

// KernelMinFree returns an estimate of the minimum amount of free memory that
// the kernel tries to maintain.  This calculation mimics the calculation of
// totalreserve_pages, which is not exported, in calculate_totalreserve_pages().
func KernelMinFree() (amountMiB uint64, err error) {
	b, err := ioutil.ReadFile("/proc/zoneinfo")
	if err != nil {
		return 0, err
	}
	v, e := kernelMinFree(string(b))
	return v, e
}

// kernelMinFree is the internal version of KernelMinFree, for unit testing.
// s is the content of /proc/zoneinfo.  Result is in MiB.
func kernelMinFree(s string) (amountMiB uint64, err error) {
	highRE := regexp.MustCompile("high\\s+(\\d+)")
	managedRE := regexp.MustCompile("managed\\s+(\\d+)")
	reserveRE := regexp.MustCompile("protection: \\((.*)\\)")
	// Parsing states.
	const (
		lookingForHigh = iota
		lookingForManaged
		lookingForProtection
		foundBoth
	)
	// Internal calculations are in pages.
	state := lookingForHigh // initial parsing state
	maxReserve := 0         // highest value in "protection" array
	var highWM int          // high watermark for zone
	var managedPages int    // managed pages for zone
	totalMinFree := 0       // total reserve

	lines := strings.Split(s, "\n")
	for _, line := range lines[:len(lines)-1] {
		groups := highRE.FindStringSubmatch(line)
		if groups != nil {
			if state != lookingForHigh {
				return 0, errors.New("field 'high' out of order in zoneinfo")
			}
			var err error
			highWM, err = strconv.Atoi(groups[1])
			if err != nil {
				return 0, errors.Wrapf(err, "bad zoneinfo 'high' field %q", groups[1])
			}
			state = lookingForManaged
			continue
		}
		groups = managedRE.FindStringSubmatch(line)
		if groups != nil {
			if state != lookingForManaged {
				return 0, errors.New("field 'managed' out of order in zoneinfo")
			}
			var err error
			managedPages, err = strconv.Atoi(groups[1])
			if err != nil {
				return 0, errors.Wrapf(err, "bad zoneinfo 'managed' field %q", groups[1])
			}
			state = lookingForProtection
			continue
		}

		groups = reserveRE.FindStringSubmatch(line)
		if groups != nil {
			if state != lookingForProtection {
				return 0, errors.New("field 'protection' out of order in zoneinfo")
			}
			fields := strings.Split(groups[1], ", ")
			for _, field := range fields {
				pages, err := strconv.Atoi(field)
				if err != nil {
					return 0, errors.Wrapf(err, "bad reserve %q", groups[1])
				}
				if maxReserve < pages {
					maxReserve = pages
				}
			}
			state = foundBoth
		}
		if state == foundBoth {
			zoneReserve := highWM + maxReserve
			if zoneReserve > managedPages {
				zoneReserve = managedPages
			}
			totalMinFree += zoneReserve
			state = lookingForHigh
		}
	}
	if state != lookingForHigh {
		return 0, errors.New("zoneinfo ended prematurely")
	}

	// Convert result from pages to MiB.
	const pageSize = 4096
	return uint64(totalMinFree) * pageSize / (1024 * 1024), nil
}

// readMemInfo returns all name-value pairs from /proc/meminfo, with the values
// in MiB rather than KiB.
func readMemInfo() (map[string]uint64, error) {
	b, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile("(\\S+):\\s+(\\d+) kB\\n")
	info := make(map[string]uint64)
	for _, groups := range re.FindAllStringSubmatch(string(b), -1) {
		v, err := strconv.ParseUint(groups[2], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "bad meminfo value: %q", groups[2])
		}
		info[groups[1]] = v
	}
	return info, nil
}

// MemInfoFields holds selected fields of /proc/meminfo.
type MemInfoFields struct {
	Total, Free, Anon, File, SwapTotal, SwapUsed uint64
}

// MemInfo returns selected /proc/meminfo fields.
func MemInfo() (data *MemInfoFields, err error) {
	info, err := readMemInfo()
	if err != nil {
		return nil, err
	}
	return &MemInfoFields{
		Total:     info["MemTotal"] / 1024,
		Free:      info["MemFree"] / 1024,
		Anon:      (info["Active(anon)"] + info["Inactive(anon)"]) / 1024,
		File:      (info["Active(file)"] + info["Inactive(file)"]) / 1024,
		SwapTotal: info["SwapTotal"] / 1024,
		SwapUsed:  (info["SwapTotal"] - info["SwapFree"]) / 1024,
	}, nil
}

// Return the numeric value of the content of filename, which is typically a
// sysfs or procfs entry.
func readUint64FromFile(filename string) (uint64, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	x, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "bad integer: %q", b)
	}
	return x, nil
}

// ChromeosLowMem returns sysfs information from the chromeos low-mem module.
func ChromeosLowMem() (availableMiB, criticalMarginMiB, ramWeight uint64, err error) {
	var v [3]uint64
	for i, f := range []string{"available", "margin", "ram_vs_swap_weight"} {
		x, err := readUint64FromFile("/sys/kernel/mm/chromeos-low_mem/" + f)
		if err != nil {
			return 0, 0, 0, err
		}
		v[i] = x
	}
	return v[0], v[1], v[2], nil
}

// ProcessMemory returns the approximate amount of virtual memory (swapped or
// not) currently allocated by processes.
func ProcessMemory() (allocatedMiB uint64, err error) {
	// Internal calculations are in KiB.
	meminfo, err := MemInfo()
	if err != nil {
		return 0, err
	}
	return (meminfo.Anon + meminfo.SwapUsed) / 1024, nil
}

// HasZram returns true when the system uses swap on a zram device.
func HasZram() bool {
	b, err := ioutil.ReadFile("/proc/swaps")
	if err != nil {
		return false
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 2 {
		return false
	}
	return strings.HasPrefix(lines[1], "/dev/zram")
}
