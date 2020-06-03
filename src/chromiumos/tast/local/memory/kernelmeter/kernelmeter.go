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
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
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
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		nameValue := strings.Split(line, " ")
		if len(nameValue) != 2 {
			panic(fmt.Sprintf("Unexpected vmstat line %q", line))
		}
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

// MemSize represents an amount of RAM in bytes.
type MemSize uint64

// String converts a MemSize to a string for printing.  The value is printed in
// MiB, since MiB resolution is more than sufficient for this application.  For
// Values smaller than 2 MiB, print a few decimals.
func (m MemSize) String() string {
	const mib = MemSize(1024 * 1024)
	if m >= 2*mib {
		return fmt.Sprintf("%d", m/mib)
	}
	return fmt.Sprintf("%.3f", float64(m)/float64(mib))
}

// watermarkData contains the sums of per-zone watermarks, plus the total
// memory reserve from the kernel.
type watermarkData struct {
	min, low, high, totalReserve MemSize
}

// watermarks returns the MM watermarks and mimics the calculation of
// totalreserve_pages, which is not exported, in
// calculate_totalreserve_pages().  The latter number is a reasonable
// approximation (and an upper bound) of the minimum amount of RAM which the
// kernel tries to keep free by reclaiming.
func watermarks() (*watermarkData, error) {
	b, err := ioutil.ReadFile("/proc/zoneinfo")
	if err != nil {
		return nil, err
	}
	return stringToWatermarks(string(b))
}

// NewMemSizePages converts a number of pages to its memory size in bytes.
func NewMemSizePages(pages int) MemSize {
	return MemSize(pages) * MemSize(os.Getpagesize())
}

// NewMemSizeKiB converts an amount in KiB to a memory size in bytes.
func NewMemSizeKiB(kib int) MemSize {
	return MemSize(kib) * 1024
}

// NewMemSizeMiB converts an amount in MiB to a memory size in bytes.
func NewMemSizeMiB(mib int) MemSize {
	return MemSize(mib) * 1024 * 1024
}

// stringToWatermarks is the internal version of watermarks, for unit testing.
// s is the content of /proc/zoneinfo.
func stringToWatermarks(s string) (*watermarkData, error) {
	watermarkRE := regexp.MustCompile(`(high|low|min)\s+(\d+)`)
	managedRE := regexp.MustCompile(`managed\s+(\d+)`)
	reserveRE := regexp.MustCompile(`protection: \((.*)\)`)
	w := &watermarkData{}
	type parseState int
	const (
		lookingForWatermarks parseState = iota
		lookingForManaged
		lookingForProtection
		foundAll
	)
	// All quantities in /proc/zoneinfo are in pages.  They are converted
	// to MemSize (bytes).
	state := lookingForWatermarks // initial parsing state
	var managed MemSize           // per-zone managed memory
	var highWM MemSize            // high watermark in a zone
	var maxReserve MemSize        // highest value in "protection" array for a zone
	var totalReserve MemSize      // total reserve, based on max per-zone reserves
	wm := map[string]MemSize{}    // values of min, low, high in a zone

	for _, line := range strings.Split(strings.TrimSuffix(string(s), "\n"), "\n") {
		if groups := watermarkRE.FindStringSubmatch(line); groups != nil {
			if state != lookingForWatermarks {
				return nil, errors.New("field out of order in zoneinfo")
			}
			var v int
			var err error
			if v, err = strconv.Atoi(groups[2]); err != nil {
				return nil, errors.Wrapf(err, "bad value %q for zoneinfo field %q", groups[2], groups[1])
			}
			wm[groups[1]] = NewMemSizePages(v)
			if len(wm) == 3 {
				w.min += wm["min"]
				w.low += wm["low"]
				w.high += wm["high"]
				highWM = w.high
				state = lookingForManaged
				wm = map[string]MemSize{} // clear watermarks map
			}
			continue
		}
		if groups := managedRE.FindStringSubmatch(line); groups != nil {
			if state != lookingForManaged {
				return nil, errors.New("field 'managed' out of order in zoneinfo")
			}
			var m int
			var err error
			if m, err = strconv.Atoi(groups[1]); err != nil {
				return nil, errors.Wrapf(err, "bad zoneinfo 'managed' field %q", groups[1])
			}
			managed = NewMemSizePages(m)
			state = lookingForProtection
			continue
		}
		if groups := reserveRE.FindStringSubmatch(line); groups != nil {
			maxReserve = 0
			if state != lookingForProtection {
				return nil, errors.New("field 'protection' out of order in zoneinfo")
			}
			for _, field := range strings.Split(groups[1], ", ") {
				r, err := strconv.Atoi(field)
				if err != nil {
					return nil, errors.Wrapf(err, "bad reserve %q", groups[1])
				}
				reserve := NewMemSizePages(r)
				if maxReserve < reserve {
					maxReserve = reserve
				}
			}
			state = foundAll
		}
		if state == foundAll {
			zoneReserve := highWM + maxReserve
			if zoneReserve > managed {
				zoneReserve = managed
			}
			totalReserve += zoneReserve
			state = lookingForWatermarks
		}
	}
	if state != lookingForWatermarks {
		return nil, errors.New("zoneinfo ended prematurely")
	}
	w.totalReserve = totalReserve
	return w, nil
}

// readMemInfo returns all name-value pairs from /proc/meminfo.  The values
// returned are in bytes.
func readMemInfo() (map[string]MemSize, error) {
	b, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`(\S+):\s+(\d+) kB\n`)
	info := make(map[string]MemSize)
	for _, groups := range re.FindAllStringSubmatch(string(b), -1) {
		v, err := strconv.Atoi(groups[2])
		if err != nil {
			return nil, errors.Wrapf(err, "bad meminfo value: %q", groups[2])
		}
		info[groups[1]] = NewMemSizeKiB(v)
	}
	return info, nil
}

// MemInfoFields holds selected fields of /proc/meminfo.
type MemInfoFields struct {
	Total, Free, Anon, File, SwapTotal, SwapUsed MemSize
}

// MemInfo returns selected /proc/meminfo fields.
func MemInfo() (data *MemInfoFields, err error) {
	info, err := readMemInfo()
	if err != nil {
		return nil, err
	}
	return &MemInfoFields{
		Total:     info["MemTotal"],
		Free:      info["MemFree"],
		Anon:      info["Active(anon)"] + info["Inactive(anon)"],
		File:      info["Active(file)"] + info["Inactive(file)"],
		SwapTotal: info["SwapTotal"],
		SwapUsed:  info["SwapTotal"] - info["SwapFree"],
	}, nil
}

// readIntFromFile returns the numeric value of the content of filename, which
// is typically a sysfs or procfs entry.
func readIntFromFile(filename string) (int, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	x, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, errors.Wrapf(err, "bad integer: %q", b)
	}
	return x, nil
}

// readFirstIntFromFile assumes filename contains one or more space-separated
// items, and returns the value of the first item which must be an integer.
func readFirstIntFromFile(filename string) (int, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, errors.Wrapf(err, "no fields in file %v", filename)
	}
	x, err := strconv.Atoi(f[0])
	if err != nil {
		return 0, errors.Wrapf(err, "bad integer: %q", f[0])
	}
	return x, nil
}

// ChromeosLowMem returns sysfs information from the chromeos low-mem module.
func ChromeosLowMem() (available, criticalMargin MemSize, ramWeight int, err error) {
	sysdir := "/sys/kernel/mm/chromeos-low_mem/"
	a, err := readIntFromFile(sysdir + "available")
	if err != nil {
		return 0, 0, 0, err
	}
	m, err := readFirstIntFromFile(sysdir + "margin")
	if err != nil {
		return 0, 0, 0, err
	}
	r, err := readIntFromFile(sysdir + "ram_vs_swap_weight")
	if err != nil {
		return 0, 0, 0, err
	}
	available = NewMemSizeMiB(a)
	criticalMargin = NewMemSizeMiB(m)
	ramWeight = r
	return
}

// ProcessMemory returns the approximate amount of virtual memory (swapped or
// not) currently allocated by processes.
func ProcessMemory() (allocated MemSize, err error) {
	meminfo, err := MemInfo()
	if err != nil {
		return 0, err
	}
	return meminfo.Anon + meminfo.SwapUsed, nil
}

// HasZram returns true when the system uses swap on a zram device,
// and no other device.
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

// LogMemoryParameters logs various kernel parameters as well as some
// calculated quantities to help understand the memory manager behavior.
func LogMemoryParameters(ctx context.Context, ratio float64) error {
	available, margin, ramWeight, err := ChromeosLowMem()
	if err != nil {
		return errors.Wrap(err, "cannot obtain low-mem info")
	}
	hasZram := HasZram()
	if !hasZram {
		// Swap to disk is the same as if the compression ratio was 0.
		ratio = 0.0
		testing.ContextLog(ctx, "Device is not using zram")
	}
	memInfo, err := MemInfo()
	if err != nil {
		return errors.Wrap(err, "cannot obtain memory info")
	}
	total := memInfo.Total
	totalSwap := memInfo.SwapTotal
	usedSwap := memInfo.SwapUsed

	// process is how much memory is in use by processes at this time.
	process, err := ProcessMemory()
	if err != nil {
		testing.ContextLog(ctx, "Cannot compute process footprint: ", err)
	}

	wm, err := watermarks()
	if err != nil {
		testing.ContextLog(ctx, "Cannot compute watermarks: ", err)
	}

	// swapReduction is the amount to be taken out of swapTotal because we
	// start discarding before swap is full.  If ramWeight is large, free
	// swap has little or no influence on available, and we assume all swap
	// space can be used.
	var swapReduction MemSize
	if margin > wm.totalReserve {
		swapReduction = (margin - wm.totalReserve) * MemSize(ramWeight)
		if swapReduction > totalSwap {
			swapReduction = 0
		}
	}
	usableSwap := totalSwap - swapReduction
	// maxProcess is the amount of allocated process memory at which the
	// low-mem device triggers.
	maxProcess := total - wm.totalReserve + MemSize(float64(usableSwap)*(1-ratio))
	if maxProcess < process {
		return errors.Errorf("bad process size calculation: max %v , current %v ", maxProcess, process)
	}
	testing.ContextLog(ctx, "Metrics: all memory sizes (RAM, swap, process) are in MiB")
	testing.ContextLogf(ctx, "Metrics: meminfo: total %v, has zram %v", total, hasZram)
	testing.ContextLogf(ctx, "Metrics: swap: total %v, used %d, usable %v", totalSwap, usedSwap, usableSwap)
	testing.ContextLogf(ctx, "Metrics: low-mem: available %v, margin %v, RAM weight %v", available, margin, ramWeight)
	testing.ContextLogf(ctx, "Metrics: watermarks %v %v %v, total reserve %v", wm.min, wm.low, wm.high, wm.totalReserve)
	testing.ContextLogf(ctx, "Metrics: process allocation: current %v, max %v, compression ratio %v", process, maxProcess, ratio)
	return nil
}
