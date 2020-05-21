// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const checkInterval = 300 * time.Millisecond
const waitUntilReady = 3 * time.Second

const totalEntryName = "total"

type loadEntry struct {
	PrivateMemory  int64   `json:"memory"`
	CPUPercent     float64 `json:"cpu_usage"`
	InTestScenario bool    `json:"in_test_scenario"`
}

// loadRecorder records the load (CPU time and memory) of the processes
// asynchronously and creates their perf reports.
type loadRecorder struct {
	procNames map[int32]string
	procs     []*process.Process

	cancel context.CancelFunc
	errorc chan error

	mutex     sync.Mutex
	recording bool
	records   map[string][]*loadEntry
}

// browserProcData searches browser-related process IDs and fill their data
// to procNames.
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
// procNames.
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
		errorc:    make(chan error),
		records:   make(map[string][]*loadEntry, len(procNames)+1),
	}
	go func() {
		defer close(lr.errorc)
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
				lr.errorc <- err
				return
			}
		}
	}()
	if err := func() error {
		if err := testing.Sleep(ctx, waitUntilReady); err != nil {
			return err
		}
		lr.mutex.Lock()
		defer lr.mutex.Unlock()
		// No data is recorded for prepares. Causing an error.
		if len(lr.records) == 0 {
			return errors.New("no records found for preparations")
		}
		return nil
	}(); err != nil {
		lr.Stop()
		return nil, err
	}
	return lr, nil
}

// Stop finishes the recording goroutine and returns the error when the
// goroutine has met an error. If it's been stopped already, do nothing and
// return nil.
func (lr *loadRecorder) Stop() error {
	if lr.cancel == nil {
		return nil
	}
	lr.cancel()
	lr.cancel = nil
	return <-lr.errorc
}

func (lr *loadRecorder) StartRecording() {
	lr.mutex.Lock()
	lr.recording = true
	lr.mutex.Unlock()
}

func (lr *loadRecorder) StopRecording() {
	lr.mutex.Lock()
	lr.recording = false
	lr.mutex.Unlock()
}

func (lr *loadRecorder) check() error {
	lr.mutex.Lock()
	recording := lr.recording
	lr.mutex.Unlock()

	for _, p := range lr.procs {
		pid := p.Pid
		name := lr.procNames[pid]
		mstat, err := p.MemoryInfoEx()
		if err != nil {
			return errors.Wrapf(err, "failed to get memory info for %d", pid)
		}
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			return errors.Wrapf(err, "failed to get CPU percent for %d", pid)
		}
		lr.records[name] = append(lr.records[name], &loadEntry{
			PrivateMemory:  int64(mstat.RSS) - int64(mstat.Shared),
			CPUPercent:     cpuPercent,
			InTestScenario: recording,
		})
	}
	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the entire CPU percent")
	}
	lr.records[totalEntryName] = append(lr.records[totalEntryName], &loadEntry{
		CPUPercent:     cpuPercents[0],
		InTestScenario: recording,
	})

	return nil
}

func (lr *loadRecorder) Save(pv *perf.Values) error {
	if lr.cancel != nil {
		return errors.New("load recorder isn't stopped yet")
	}
	for name, records := range lr.records {
		if name == totalEntryName {
			// Recording the TPS score of CPU.
			// 0.5 * mean CPU usage + 0.3 * 95-percentile usage + 0.2 * 75-percentile usage.
			percents := make([]float64, 0, len(records))
			for _, data := range records {
				if data.InTestScenario {
					percents = append(percents, data.CPUPercent)
				}
			}
			sort.Float64s(percents)
			var sum, sum95, sum75 float64
			var count95, count75 int
			for i := 0; i < len(percents); i++ {
				sum += percents[i]
				if i < len(percents)*19/20 {
					sum95 += percents[i]
					count95++
					if i < len(percents)*3/4 {
						sum75 += percents[i]
						count75++
					}
				}
			}
			mean := sum / float64(len(percents))
			pv.Set(perf.Metric{
				Name:      "TPS.CPU",
				Variant:   "average",
				Unit:      "score",
				Direction: perf.SmallerIsBetter,
			}, mean)
			score95 := sum95 / float64(count95)
			pv.Set(perf.Metric{
				Name:      "TPS.CPU",
				Variant:   "95percentile",
				Unit:      "score",
				Direction: perf.SmallerIsBetter,
			}, score95)
			score75 := sum75 / float64(count75)
			pv.Set(perf.Metric{
				Name:      "TPS.CPU",
				Variant:   "75percentile",
				Unit:      "score",
				Direction: perf.SmallerIsBetter,
			}, score75)
			pv.Set(perf.Metric{
				Name:      "TPS.CPU",
				Unit:      "score",
				Direction: perf.SmallerIsBetter,
			}, mean*0.5+score95*0.3+score75*0.2)
			continue
		}
		var prepareMax *loadEntry
		for _, data := range records {
			if data.InTestScenario {
				break
			}
			if prepareMax != nil {
				if prepareMax.PrivateMemory < data.PrivateMemory {
					prepareMax.PrivateMemory = data.PrivateMemory
				}
				prepareMax.CPUPercent = math.Max(prepareMax.CPUPercent, data.CPUPercent)
			} else {
				prepareMax = &loadEntry{PrivateMemory: data.PrivateMemory, CPUPercent: data.CPUPercent}
			}
		}
		var sum loadEntry
		var maxIncrease loadEntry
		for _, data := range records {
			if !data.InTestScenario {
				continue
			}
			sum.PrivateMemory += data.PrivateMemory
			sum.CPUPercent += data.CPUPercent
			increase := &loadEntry{
				PrivateMemory: data.PrivateMemory - prepareMax.PrivateMemory,
				CPUPercent:    data.CPUPercent - prepareMax.CPUPercent,
			}
			if maxIncrease.PrivateMemory < increase.PrivateMemory {
				maxIncrease.PrivateMemory = increase.PrivateMemory
			}
			maxIncrease.CPUPercent = math.Max(maxIncrease.CPUPercent, increase.CPUPercent)
		}

		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.cpuPercent", name),
			Variant:   "average",
			Direction: perf.SmallerIsBetter,
			Unit:      "percent",
		}, sum.CPUPercent/float64(len(lr.records)))
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.cpuPercent", name),
			Variant:   "maxIncrease",
			Direction: perf.SmallerIsBetter,
			Unit:      "percent",
		}, maxIncrease.CPUPercent)
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.privateMemory", name),
			Variant:   "average",
			Direction: perf.SmallerIsBetter,
			Unit:      "bytes",
		}, float64(sum.PrivateMemory)/float64(len(lr.records)))
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.privateMemory", name),
			Variant:   "maxIncrease",
			Direction: perf.SmallerIsBetter,
			Unit:      "bytes",
		}, float64(maxIncrease.PrivateMemory))
	}

	return nil
}

func (lr *loadRecorder) SaveRecords(outDir string) error {
	if lr.cancel != nil {
		return errors.New("load recorder isn't stopped yet")
	}
	records := lr.records[totalEntryName]
	jsonData, err := json.Marshal(map[string]interface{}{"records": records})
	if err != nil {
		return errors.Wrap(err, "failed to marshal json")
	}
	return ioutil.WriteFile(filepath.Join(outDir, "cpu-records.json"), jsonData, 0644)
}
