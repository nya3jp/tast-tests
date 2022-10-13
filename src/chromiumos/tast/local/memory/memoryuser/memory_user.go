// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs.
package memoryuser

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/v3/mem"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

// TestEnv is a struct containing the data to be used across the test.
type TestEnv struct {
	wpr       *wpr.WPR
	cr        *chrome.Chrome
	br        *browser.Browser
	brCleanup func(ctx context.Context) error
	arc       *arc.ARC
	// tconn is to Ash Chrome.
	tconn  *chrome.TestConn
	p      *perf.Values
	vm     bool
	outDir string
}

// MemoryTask describes a memory-consuming task to perform.
// This allows various types of activities, such as ARC, Chrome, and VMs, to be defined
// using the same setup and run in parallel.
type MemoryTask interface {
	// Run performs the memory-related task.
	Run(ctx context.Context, testEnv *TestEnv) error
	// Close closes any initialized data and connections for the memory-related task.
	Close(ctx context.Context, testEnv *TestEnv)
	// String returns a string describing the memory-related task.
	String() string
	// NeedVM returns a bool indicating whether the task needs a VM
	NeedVM() bool
}

// logCmd logs the output of the provided command into a file every 5 seconds.
func logCmd(ctx context.Context, outfile, cmdStr string, args ...string) {
	file, err := os.Create(outfile)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to create log file %s: %v", outfile, err)
		return
	}
	defer file.Close()
	for {
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return
		}

		fmt.Fprintf(file, "%s\n", time.Now())

		cmd := testexec.CommandContext(ctx, cmdStr, args...)
		cmd.Stdout = file
		cmd.Stderr = file
		if err := cmd.Run(); err != nil {
			testing.ContextLogf(ctx, "Command %s failed: %v", cmdStr, err)
			return
		}
	}
}

// copyMemdLogs copies the memd log files into the output directory of the test.
func copyMemdLogs(outDir string) error {
	const inputDir = "/var/log/memd"
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		return errors.Wrap(err, "cannot read memd")
	}

	for _, file := range files {
		inputFile := filepath.Join(inputDir, file.Name())
		if err = fsutil.CopyFile(inputFile, filepath.Join(outDir, file.Name())); err != nil {
			return errors.Wrap(err, "cannot copy memd file")
		}
	}
	return nil
}

// prepareMemdLogging restarts memd and removes old memd log files.
func prepareMemdLogging(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, "memd"); err != nil {
		return errors.Wrap(err, "cannot restart memd")
	}
	const clipFilesPattern = "/var/log/memd/memd.clip*.log"
	// Remove any clip files from /var/log/memd.
	files, err := filepath.Glob(clipFilesPattern)
	if err != nil {
		return errors.Wrapf(err, "cannot list %v", clipFilesPattern)
	}
	for _, file := range files {
		if err = os.Remove(file); err != nil {
			return errors.Wrapf(err, "cannot remove %v", file)
		}
	}
	return nil
}

// resetAndLogStats logs the VM stats from the provided kernelmeter with the identifying label,
// then resets the meter.
func resetAndLogStats(ctx context.Context, meter *kernelmeter.Meter, label string) {
	defer meter.Reset()
	stats, err := meter.VMStats()
	if err != nil {
		testing.ContextLogf(ctx, "Metrics: %s: could not log stats for test: %v", label, err)
		return
	}
	testing.ContextLogf(ctx, "Metrics: %s: total page fault count %d", label, stats.PageFault.Count)
	testing.ContextLogf(ctx, "Metrics: %s: average page fault rate %.1f pf/second", label, stats.PageFault.AverageRate)
	testing.ContextLogf(ctx, "Metrics: %s: max page fault rate %.1f pf/second", label, stats.PageFault.MaxRate)
	testing.ContextLogf(ctx, "Metrics: %s: total swap-in count %d", label, stats.SwapIn.Count)
	testing.ContextLogf(ctx, "Metrics: %s: average swap-in rate %.1f swaps/second", label, stats.SwapIn.AverageRate)
	testing.ContextLogf(ctx, "Metrics: %s: max swap-in rate %.1f swaps/second", label, stats.SwapIn.MaxRate)
	testing.ContextLogf(ctx, "Metrics: %s: total swap-out count %d", label, stats.SwapOut.Count)
	testing.ContextLogf(ctx, "Metrics: %s: average swap-out rate %.1f swaps/second", label, stats.SwapOut.AverageRate)
	testing.ContextLogf(ctx, "Metrics: %s: max swap-out rate %.1f swaps/second", label, stats.SwapOut.MaxRate)
	testing.ContextLogf(ctx, "Metrics: %s: total OOM count %d", label, stats.OOM.Count)
	if swapInfo, err := mem.SwapMemory(); err == nil {
		testing.ContextLogf(ctx, "Metrics: %s: free swap %v MiB", label, (swapInfo.Total-swapInfo.Used)/(1<<20))
	}
	if availableMiB, _, _, err := kernelmeter.ChromeosLowMem(); err == nil {
		testing.ContextLogf(ctx, "Metrics: %s: available %v MiB", label, availableMiB)
	}
	if m, err := kernelmeter.MemInfo(); err == nil {
		testing.ContextLogf(ctx, "Metrics: %s: free %v MiB, anon %v MiB, file %v MiB", label, m.Free, m.Anon, m.File)
	}
}

// setPerfValues sets values for perf metrics from kernelmeter data
func setPerfValues(meter *kernelmeter.Meter, values *perf.Values, label string) error {
	stats, err := meter.VMStats()
	if err != nil {
		return errors.Wrapf(err, "cannot compute page fault stats (%s)", label)
	}
	totalPageFaultCountMetric := perf.Metric{
		Name:      "tast_total_page_fault_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averagePageFaultRateMetric := perf.Metric{
		Name:      "tast_average_page_fault_rate_" + label,
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	maxPageFaultRateMetric := perf.Metric{
		Name:      "tast_max_page_fault_rate_" + label,
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	totalSwapInCountMetric := perf.Metric{
		Name:      "tast_total_swap_in_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averageSwapInRateMetric := perf.Metric{
		Name:      "tast_average_swap_in_rate_" + label,
		Unit:      "swaps_per_second",
		Direction: perf.SmallerIsBetter,
	}
	maxSwapInRateMetric := perf.Metric{
		Name:      "tast_max_swap_in_rate_" + label,
		Unit:      "swaps_per_second",
		Direction: perf.SmallerIsBetter,
	}
	totalSwapOutCountMetric := perf.Metric{
		Name:      "tast_total_swap_out_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averageSwapOutRateMetric := perf.Metric{
		Name:      "tast_average_swap_out_rate_" + label,
		Unit:      "swaps_per_second",
		Direction: perf.SmallerIsBetter,
	}
	maxSwapOutRateMetric := perf.Metric{
		Name:      "tast_max_swap_out_rate_" + label,
		Unit:      "swaps_per_second",
		Direction: perf.SmallerIsBetter,
	}
	totalOOMCountMetric := perf.Metric{
		Name:      "tast_total_oom_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	values.Set(totalPageFaultCountMetric, float64(stats.PageFault.Count))
	values.Set(averagePageFaultRateMetric, stats.PageFault.AverageRate)
	values.Set(maxPageFaultRateMetric, stats.PageFault.MaxRate)
	values.Set(totalSwapInCountMetric, float64(stats.SwapIn.Count))
	values.Set(averageSwapInRateMetric, stats.SwapIn.AverageRate)
	values.Set(maxSwapInRateMetric, stats.SwapIn.MaxRate)
	values.Set(totalSwapOutCountMetric, float64(stats.SwapOut.Count))
	values.Set(averageSwapOutRateMetric, stats.SwapOut.AverageRate)
	values.Set(maxSwapOutRateMetric, stats.SwapOut.MaxRate)
	values.Set(totalOOMCountMetric, float64(stats.OOM.Count))
	return nil
}

// initChrome starts the Chrome browser.
func initChrome(ctx context.Context, p *RunParameters, te *TestEnv) error {
	var opts []chrome.Option
	var err error

	if p.ExistingChrome != nil {
		te.cr = p.ExistingChrome
		if p.WPRArchivePath != "" {
			return errors.New("WPR can not be used with ExistingChrome")
		}
		return nil
	}

	if p.WPRArchivePath != "" {
		te.wpr, err = wpr.New(ctx, p.WPRMode, p.WPRArchivePath)
		if err != nil {
			return errors.Wrap(err, "cannot start WPR")
		}

		opts = append(opts, te.wpr.ChromeOptions...)
	}

	if p.UseARC {
		opts = append(opts, chrome.ARCEnabled())
	}

	te.cr, err = chrome.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "cannot start chrome")
	}

	return nil
}

// newTestEnv creates a new TestEnv, creating new Chrome, ARC, ARC UI Automator device,
// and VM instances to use.
func newTestEnv(ctx context.Context, outDir string, p *RunParameters) (*TestEnv, error) {
	te := &TestEnv{
		vm:     false,
		outDir: outDir,
		p:      p.ExistingPerfValues,
	}

	if te.p == nil {
		te.p = perf.NewValues()
	}

	// Schedule closure of partially-initialized struct.
	toClose := te
	defer func() {
		if toClose != nil {
			toClose.Close(ctx, p)
		}
	}()

	if err := initChrome(ctx, p, te); err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}

	if p.BrowserType != "" {
		br, cleanup, err := browserfixt.SetUp(ctx, te.cr, p.BrowserType)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get %q Browser", p.BrowserType)
		}
		te.br = br
		te.brCleanup = cleanup
	} else {
		te.br = te.cr.Browser()
	}

	var err error
	if p.UseARC {
		if p.ExistingARC != nil {
			te.arc = p.ExistingARC
		} else if te.arc, err = arc.New(ctx, outDir); err != nil {
			return nil, errors.Wrap(err, "failed to start ARC")
		}
	}

	if te.tconn, err = te.cr.TestAPIConn(ctx); err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}

	toClose = nil
	return te, nil
}

func startVM(ctx context.Context, te *TestEnv) error {
	if te.vm {
		return nil
	}
	testing.ContextLog(ctx, "Waiting for crostini to install (typically ~ 3 mins) and mount sshfs")
	if err := te.tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.runCrostiniInstaller)`); err != nil {
		return errors.Wrap(err, "autotestPrivate.runCrostiniInstaller failed")
	}
	te.vm = true
	return nil
}

// runTask runs a MemoryTask.
func runTask(ctx, taskCtx context.Context, task MemoryTask, te *TestEnv) error {
	taskMeter := kernelmeter.New(ctx)
	defer taskMeter.Close(ctx)
	if task.NeedVM() {
		if err := startVM(ctx, te); err != nil {
			return errors.Wrap(err, "failed to start VM")
		}
	}
	if err := task.Run(taskCtx, te); err != nil {
		return errors.Wrapf(err, "failed to run memory task %s", task.String())
	}
	resetAndLogStats(ctx, taskMeter, task.String())
	return nil
}

// RunParameters contains the configurable parameters for RunTest.
type RunParameters struct {
	// WPRMode is the mode to start WPR.
	WPRMode wpr.Mode
	// WPRArchivePath is the full path to an archive for WPR. If set, WPR is used
	// and Chrome sends its traffic through WPR. Otherwise, Chrome uses live
	// sites.
	WPRArchivePath string
	// UseARC indicates whether Chrome should be started with ARC enabled.
	UseARC bool
	// ParallelTasks indicates whether the memory tasks should be run in parallel
	ParallelTasks bool
	// ExistingChrome indicates that we should use this Chrome instance instead
	// of creating a new one.
	ExistingChrome *chrome.Chrome
	// BrowserType indicates which browser should be used.
	BrowserType browser.Type
	// ExistingARC indicates that we should use this ARC instance instead of
	// creating a new one. ExistingChrome and UseARC must be set.
	ExistingARC *arc.ARC
	// ExistingPerfValues indicates that we should use this perf.Values instead
	// of creating a new one. The caller is responsible calling
	// perf.Values.Save().
	ExistingPerfValues *perf.Values
}

// Close closes the Chrome, ARC, and WPR instances used in the TestEnv.
func (te *TestEnv) Close(ctx context.Context, p *RunParameters) {
	if te.vm {
		if err := te.tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.runCrostiniUninstaller)`); err != nil {
			testing.ContextLog(ctx, "Running autotestPrivate.runCrostiniInstaller failed: ", err)
		}
	}
	if te.arc != nil && p.ExistingARC == nil {
		te.arc.Close(ctx)
		te.arc = nil
	}
	if te.brCleanup != nil {
		if err := te.brCleanup(ctx); err != nil {
			testing.ContextLog(ctx, "Running browser.Browser cleanup method failed: ", err)
		}
		te.brCleanup = nil
	}
	te.br = nil
	if te.cr != nil && p.ExistingChrome == nil {
		te.cr.Close(ctx)
		te.cr = nil
	}
	if te.wpr != nil {
		te.wpr.Close(ctx)
		te.wpr = nil
	}
}

// RunTest creates a new TestEnv and then runs ARC, Chrome, and VM tasks in parallel.
// It also logs memory and cpu usage throughout the test, and copies output from /var/log/memd and /var/log/vmlog
// when finished.
// All passed-in tasks will be closed automatically.
func RunTest(ctx context.Context, outDir string, tasks []MemoryTask, p *RunParameters) (errRet error) {
	if p == nil {
		p = &RunParameters{}
	}
	testEnv, err := newTestEnv(ctx, outDir, p)
	if err != nil {
		return errors.Wrap(err, "failed creating the test environment")
	}
	defer testEnv.Close(ctx, p)

	if err = prepareMemdLogging(ctx); err != nil {
		return errors.Wrap(err, "failed to prepare memd logging")
	}
	go logCmd(ctx, filepath.Join(outDir, "memory_use.txt"), "cat", "/proc/meminfo")
	go logCmd(ctx, filepath.Join(outDir, "cpu_use.txt"), "iostat", "-c")

	testMeter := kernelmeter.New(ctx)
	defer testMeter.Close(ctx)

	taskCtx, taskCancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer taskCancel()

	if p.ParallelTasks {
		ch := make(chan struct{}, len(tasks))
		for _, task := range tasks {
			go func(task MemoryTask) {
				defer func() {
					task.Close(ctx, testEnv)
					ch <- struct{}{}
				}()
				err = runTask(ctx, taskCtx, task, testEnv)
				if err != nil {
					testing.ContextLog(ctx, "Failed to run task: ", err)
					if errRet == nil {
						errRet = errors.Wrap(err, "failed to run task")
					}
				}
			}(task)
		}
		for i := 0; i < len(tasks); i++ {
			select {
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "tasks didn't complete")
			case <-ch:
			}
		}
	} else {
		for _, task := range tasks {
			err = runTask(ctx, taskCtx, task, testEnv)
			if err != nil {
				return errors.Wrap(err, "failed to run task")
			}
		}
	}

	const vmlog = "/var/log/vmlog/vmlog.LATEST"
	if err := fsutil.CopyFile(vmlog, filepath.Join(outDir, filepath.Base(vmlog))); err != nil {
		return errors.Wrapf(err, "failed to copy %v", vmlog)
	}
	if err = copyMemdLogs(outDir); err != nil {
		return errors.Wrap(err, "failed to get memd files")
	}

	setPerfValues(testMeter, testEnv.p, "full_test")
	resetAndLogStats(ctx, testMeter, "full test")
	if p.ExistingPerfValues == nil {
		if err = testEnv.p.Save(outDir); err != nil {
			return errors.Wrap(err, "cannot save perf data")
		}
	}
	// NB: errRet can be set by goroutines above, so don't override it if they
	// had an error. Those errors are also logged, so if we have an early return
	// because of another error, they should still be diagnosable.
	return errRet
}
