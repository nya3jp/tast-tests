// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
)

type recorderState int

const (
	recorderInit recorderState = iota
	recorderInactive
	recorderRecording
)

const checkInterval = 100 * time.Millisecond
const waitUntilReady = 3 * time.Second

const totalEntryName = "total"

type loadEntry struct {
	privateMemory int64
	cpuPercent    float64
}

// loadRecorder records the load (CPU time and memory) of the browsers
// asynchronously and creates their perf reports.
type loadRecorder struct {
	mutex sync.Mutex

	procNames map[int32]string
	procs     []*process.Process

	state     recorderState
	cancel    context.CancelFunc
	readyc    chan struct{}
	lastError error

	prepares []map[string]*loadEntry
	records  []map[string]*loadEntry
}

// browserProcData searches browser-related process IDs and fill their data
// to |procNames|.
func browserProcData(procNames map[int32]string) error {
	browserPID, err := chrome.GetRootPID()
	if err != nil {
		return errors.Wrap(err, "failed to find the browser process")
	}
	gpuProcs, err := chrome.GetGPUProcesses()
	if err != nil {
		return errors.Wrap(err, "failed to find the GPU process")
	}
	if len(gpuProcs) != 1 {
		return errors.Errorf("found %d GPU processes, expected to have one", len(gpuProcs))
	}
	procNames[int32(browserPID)] = "browser"
	procNames[gpuProcs[0].Pid] = "gpu"
	return nil
}

// arcProcData searches ARC-related process IDs and fill their data to
// |procNames|.
func arcProcData(procNames map[int32]string) error {
	procs, err := process.Processes()
	if err != nil {
		return errors.Wrap(err, "failed to get the process list")
	}
	for _, proc := range procs {
		cmdline, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if cmdline == "system_server" {
			procNames[proc.Pid] = "system_server"
			return nil
		}
	}
	// It's fine the system_server does not exist; ARC might not be activated,
	// or it's ARCVM and we can't check its load.
	// TODO(mukai): consider the case of ARCVM.
	return nil
}

// newLoadRecorder creates a new loadRecorder.
func newLoadRecorder(ctx context.Context, procNames map[int32]string) (*loadRecorder, error) {
	var procs []*process.Process
	for pid, name := range procNames {
		p, err := process.NewProcess(pid)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find the process %d (%s)", pid, name)
		}
		procs = append(procs, p)
	}

	// before dispatching all CPU state; invokes cpu.Percent to reset the 'last'
	// state.
	if _, err := cpu.Percent(0, false); err != nil {
		return nil, errors.Wrap(err, "failed to compute the entire CPU percent")
	}
	ctx, cancel := context.WithCancel(ctx)
	lr := &loadRecorder{
		procNames: procNames,
		procs:     procs,
		cancel:    cancel,
		readyc:    make(chan struct{}),
	}
	go func() {
		<-time.After(waitUntilReady)
		lr.mutex.Lock()
		lr.state = recorderInactive
		lr.mutex.Unlock()
		close(lr.readyc)
	}()
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			err := lr.check()
			if err != nil {
				return
			}
		}
	}()
	return lr, nil
}

func (lr *loadRecorder) WaitUntilReady() {
	<-lr.readyc
}

// Close finishes the recording goroutine and returns the error when the
// goroutine has met an error.
func (lr *loadRecorder) Close() error {
	lr.cancel()
	lr.mutex.Lock()
	defer lr.mutex.Unlock()
	return lr.lastError
}

func (lr *loadRecorder) StartRecording() {
	lr.mutex.Lock()
	lr.state = recorderRecording
	lr.mutex.Unlock()
}

func (lr *loadRecorder) StopRecording() {
	lr.mutex.Lock()
	lr.state = recorderInactive
	lr.mutex.Unlock()
}

func (lr *loadRecorder) check() error {
	lr.mutex.Lock()
	defer lr.mutex.Unlock()
	if lr.state == recorderInactive {
		return nil
	}

	record := make(map[string]*loadEntry, len(lr.procs)+1)
	for _, p := range lr.procs {
		pid := p.Pid
		name := lr.procNames[pid]
		mstat, err := p.MemoryInfoEx()
		if err != nil {
			lr.lastError = errors.Wrapf(err, "failed to get memory info for %d", pid)
			return lr.lastError
		}
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			lr.lastError = errors.Wrapf(err, "failed to get CPU percent for %d", pid)
			return lr.lastError
		}
		record[name] = &loadEntry{
			privateMemory: int64(mstat.RSS) - int64(mstat.Shared),
			cpuPercent:    cpuPercent,
		}
	}
	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		lr.lastError = errors.Wrap(err, "failed to obtain the entire CPU percent")
		return lr.lastError
	}
	record[totalEntryName] = &loadEntry{cpuPercent: cpuPercents[0]}

	if lr.state == recorderInit {
		lr.prepares = append(lr.prepares, record)
	} else {
		lr.records = append(lr.records, record)
	}
	return nil
}

func (lr *loadRecorder) Record(pv *perf.Values) error {
	if err := lr.Close(); err != nil {
		return err
	}

	prepareMaxes := make(map[string]*loadEntry, len(lr.procNames)+1)
	for _, record := range lr.prepares {
		for name, data := range record {
			pm, ok := prepareMaxes[name]
			if ok {
				if pm.privateMemory < data.privateMemory {
					pm.privateMemory = data.privateMemory
				}
				pm.cpuPercent = math.Max(pm.cpuPercent, data.cpuPercent)
			} else {
				prepareMaxes[name] = &loadEntry{privateMemory: data.privateMemory, cpuPercent: data.cpuPercent}
			}
		}
	}
	avg := make(map[string]*loadEntry, len(lr.procNames)+1)
	maxIncreases := make(map[string]*loadEntry, len(lr.procNames)+1)
	for _, record := range lr.records {
		for name, data := range record {
			if avgStat, ok := avg[name]; ok {
				avgStat.privateMemory += data.privateMemory
				avgStat.cpuPercent += data.cpuPercent
			} else {
				avg[name] = &loadEntry{privateMemory: data.privateMemory, cpuPercent: data.cpuPercent}
			}
			prepareMax := prepareMaxes[name]
			increase := &loadEntry{
				privateMemory: data.privateMemory - prepareMax.privateMemory,
				cpuPercent:    data.cpuPercent - prepareMax.cpuPercent,
			}
			if maxIncrease, ok := maxIncreases[name]; ok {
				if maxIncrease.privateMemory < increase.privateMemory {
					maxIncrease.privateMemory = increase.privateMemory
				}
				maxIncrease.cpuPercent = math.Max(maxIncrease.cpuPercent, increase.cpuPercent)
			} else {
				maxIncreases[name] = increase
			}
		}
	}
	for name, avgStat := range avg {
		if name != totalEntryName {
			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.privateMemory", name),
				Variant:   "average",
				Direction: perf.SmallerIsBetter,
				Unit:      "bytes",
			}, float64(avgStat.privateMemory)/float64(len(lr.records)))
		}
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.cpuPercent", name),
			Variant:   "average",
			Direction: perf.SmallerIsBetter,
			Unit:      "percent",
		}, avgStat.cpuPercent/float64(len(lr.records)))
	}
	for name, maxIncrease := range maxIncreases {
		if name != totalEntryName {
			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("%s.privateMemory", name),
				Variant:   "maxIncrease",
				Direction: perf.SmallerIsBetter,
				Unit:      "bytes",
			}, float64(maxIncrease.privateMemory))
		}
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.cpuPercent", name),
			Variant:   "maxIncrease",
			Direction: perf.SmallerIsBetter,
			Unit:      "percent",
		}, maxIncrease.cpuPercent)
	}
	return nil
}
