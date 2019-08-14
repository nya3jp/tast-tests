// Copyright 2019 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// TestEnv is a struct containing the Chrome Instance, ARC instance,
// ARC UI Automator device, and VM to be used across the test.
type TestEnv struct {
	chrome    *chrome.Chrome
	arc       *arc.ARC
	arcDevice *ui.Device
	tconn     *chrome.Conn
	vm        bool
	wpr       *testexec.Cmd
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

// initChrome starts the Chrome browser.
func initChrome(ctx context.Context, p *RunParameters) (*chrome.Chrome, *testexec.Cmd, error) {
	if p.MemoryPressureWPR {
		if p.MemoryPressureParameters == nil {
			p.MemoryPressureParameters = &mempressure.RunParameters{}
		}
		if p.UseARC {
			p.MemoryPressureParameters.UseARC = true
		}
		cr, wpr, err := mempressure.InitBrowser(ctx, p.MemoryPressureParameters)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot start Chrome")
		}
		return cr, wpr, err
	}
	var cr *chrome.Chrome
	var err error
	if p.UseARC {
		cr, err = chrome.New(ctx, chrome.ARCEnabled())
	} else {
		cr, err = chrome.New(ctx)
	}
	return cr, nil, err

}

// newTestEnv creates a new TestEnv, creating new Chrome, ARC, ARC UI Automator device,
// and VM instances to use.
func newTestEnv(ctx context.Context, outDir string, p *RunParameters) (*TestEnv, error) {
	te := &TestEnv{
		vm: false,
	}

	// Schedule closure of partially-initialized struct.
	toClose := te
	defer func() {
		if toClose != nil {
			toClose.Close(ctx)
		}
	}()

	var err error
	if te.chrome, te.wpr, err = initChrome(ctx, p); err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}

	if te.arc, err = arc.New(ctx, outDir); err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}

	if te.arcDevice, err = ui.NewDevice(ctx, te.arc); err != nil {
		return nil, errors.Wrap(err, "failed initializing UI Automator")
	}

	if te.tconn, err = te.chrome.TestAPIConn(ctx); err != nil {
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
	if err := te.tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.runCrostiniInstaller(() => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(new Error(chrome.runtime.lastError.message));
		     }
		   });
		})`, nil); err != nil {
		return errors.Wrap(err, "Running autotestPrivate.runCrostiniInstaller failed")
	}
	te.vm = true
	return nil
}

// Close closes the Chrome, ARC, ARC UI Automator device, and VM instances used in the TestEnv.
func (te *TestEnv) Close(ctx context.Context) {
	if te.vm {
		if err := te.tconn.EvalPromise(ctx,
			`new Promise((resolve, reject) => {
		chrome.autotestPrivate.runCrostiniUninstaller(() => {
		  if (chrome.runtime.lastError === undefined) {
		    resolve();
		  } else {
		    reject(new Error(chrome.runtime.lastError.message));
		  }
		});
	})`, nil); err != nil {
			testing.ContextLog(ctx, "Running autotestPrivate.runCrostiniInstaller failed: ", err)
		}
	}
	if te.arcDevice != nil {
		te.arcDevice.Close()
	}
	if te.arc != nil {
		te.arc.Close()
	}
	if te.wpr != nil {
		te.wpr.Process.Signal(os.Interrupt)
	}
	if te.chrome != nil {
		te.chrome.Close(ctx)
	}
}

// RunParameters contains the configurable parameters for RunTest
type RunParameters struct {
	// MemoryPressureWPR indicates whether the memory pressure test with
	// WPR will be run, which will determine what arguments are needed
	// when starting chrome
	MemoryPressureWPR bool
	// MemoryPressureParameters is the RunParameters for a Memory Pressure Task
	// to include when starting Chrome
	MemoryPressureParameters *mempressure.RunParameters
	// UseARC indicates whether Chrome should be started with ARC enabled. This
	// is needed for running AndroidTasks
	UseARC bool
}

// RunTest creates a new TestEnv and then runs ARC, Chrome, and VM tasks in parallel.
// It also logs memory and cpu usage throughout the test, and copies output from /var/log/memd and /var/log/vmlog
// when finished.
// All passed-in tasks will be closed automatically.
func RunTest(ctx context.Context, s *testing.State, tasks []MemoryTask, p *RunParameters) {
	if p == nil {
		p = &RunParameters{}
	}
	testEnv, err := newTestEnv(ctx, s.OutDir(), p)
	if err != nil {
		s.Fatal("Failed creating the test environment: ", err)
	}
	defer testEnv.Close(ctx)

	if err = prepareMemdLogging(ctx); err != nil {
		s.Error("Failed to prepare memd logging: ", err)
	}
	go logCmd(ctx, filepath.Join(s.OutDir(), "memory_use.txt"), "cat", "/proc/meminfo")
	go logCmd(ctx, filepath.Join(s.OutDir(), "cpu_use.txt"), "iostat", "-c")

	taskCtx, taskCancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer taskCancel()

	ch := make(chan struct{}, len(tasks))
	for _, task := range tasks {
		go func(task MemoryTask) {
			defer func() {
				task.Close(ctx, testEnv)
				ch <- struct{}{}
			}()
			if task.NeedVM() {
				if err := startVM(ctx, testEnv); err != nil {
					s.Error("Failed to start VM: ", err)
					return
				}
			}
			if err := task.Run(taskCtx, testEnv); err != nil {
				s.Errorf("Failed to run memory task %s: %v", task.String(), err)
			}
		}(task)
	}
	for i := 0; i < len(tasks); i++ {
		select {
		case <-ctx.Done():
			s.Error("Tasks didn't complete: ", ctx.Err())
		case <-ch:
		}
	}

	const vmlog = "/var/log/vmlog/vmlog.LATEST"
	if err := fsutil.CopyFile(vmlog, filepath.Join(s.OutDir(), filepath.Base(vmlog))); err != nil {
		s.Errorf("Failed to copy %v: %v", vmlog, err)
	}
	if err = copyMemdLogs(s.OutDir()); err != nil {
		s.Error("Failed to get memd files: ", err)
	}
}
