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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// TestEnv is a struct containing the Chrome Instance, ARC instance,
// ARC UI Automator device, and VM to be used across the test.
type TestEnv struct {
	chrome    *chrome.Chrome
	arc       *arc.ARC
	arcDevice *ui.Device
	vm        *vm.VM
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

// newTestEnv creates a new TestEnv, creating new Chrome, ARC, ARC UI Automator device,
// and VM instances to use.
func newTestEnv(ctx context.Context, outDir string) (*TestEnv, error) {
	te := &TestEnv{}

	// Schedule closure of partially-initialized struct.
	toClose := te
	defer func() {
		if toClose != nil {
			toClose.Close(ctx)
		}
	}()

	var err error
	if te.chrome, err = chrome.New(ctx, chrome.ARCEnabled()); err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}

	if te.arc, err = arc.New(ctx, outDir); err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}

	if te.arcDevice, err = ui.NewDevice(ctx, te.arc); err != nil {
		return nil, errors.Wrap(err, "failed initializing UI Automator")
	}

	var tconn *chrome.Conn
	if tconn, err = te.chrome.TestAPIConn(ctx); err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to enable Crostini preference setting")
	}
	if err = vm.SetUpComponent(ctx, vm.StagingComponent); err != nil {
		return nil, errors.Wrap(err, "failed to set up component")
	}

	var conc *vm.Concierge
	if conc, err = vm.NewConcierge(ctx, te.chrome.User()); err != nil {
		return nil, errors.Wrap(err, "failed to start concierge")
	}

	te.vm = vm.NewDefaultVM(conc)
	if err = te.vm.Start(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start VM")
	}

	toClose = nil
	return te, nil

}

// Close closes the Chrome, ARC, ARC UI Automator device, and VM instances used in the TestEnv.
func (te *TestEnv) Close(ctx context.Context) {
	if te.vm != nil {
		te.vm.Stop(ctx)
	}
	if te.arcDevice != nil {
		te.arcDevice.Close()
	}
	if te.arc != nil {
		te.arc.Close()
	}
	if te.chrome != nil {
		te.chrome.Close(ctx)
	}
}

// RunTest creates a new TestEnv and then runs ARC, Chrome, and VM tasks in parallel.
// It also logs memory and cpu usage throughout the test, and copies output from /var/log/memd and /var/log/vmlog
// when finished.
// All passed-in tasks will be closed automatically.
func RunTest(ctx context.Context, s *testing.State, tasks []MemoryTask) {
	testEnv, err := newTestEnv(ctx, s.OutDir())
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
			if err := task.Run(taskCtx, testEnv); err != nil {
				s.Errorf("Failed to run memory task %s: %v", task.String(), err)
			}
			defer task.Close(ctx, testEnv)
			ch <- struct{}{}
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
