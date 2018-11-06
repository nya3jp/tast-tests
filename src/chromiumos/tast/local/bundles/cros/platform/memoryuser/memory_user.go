// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs.
package memoryuser

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

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
	Chrome    *chrome.Chrome
	ARC       *arc.ARC
	ARCDevice *ui.Device
	VM        *vm.VM
}

// MemoryTask is an interface with the method Run which takes a TestEnv.
// This allows various types of activities, such as ARC, Chrome, and VMs, to be defined
// using the same setup and run in parallel.
type MemoryTask interface {
	// Run performs the memory-related task.
	Run(ctx context.Context, testEnv *TestEnv) error
	// Close closes any initialized data and connections for the memory-related task.
	Close(ctx context.Context, testEnv *TestEnv)
}

// logUse logs the output of the provided command into a file every 5 seconds.
func logUse(ctx context.Context, command, arg, filename string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to create log file: %s", filename, err)
	}
	defer file.Close()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			b, err := testexec.CommandContext(ctx, "date").CombinedOutput()
			if err != nil {
				return errors.Wrap(err, "date command failed")
			}
			if _, err := file.Write(b); err != nil {
				return errors.Wrapf(err, "writing to %s failed", filename)
			}

			b, err = testexec.CommandContext(ctx, command, arg).CombinedOutput()
			if err != nil {
				return errors.Wrapf(err, "command %s %s failed", command, arg)
			}
			if _, err := file.Write(b); err != nil {
				return errors.Wrapf(err, "writing to %s failed", filename)
			}
		}
	}
}

// getVMLog copies the file vmlog.LATEST to the output directory of the test.
func getVMLog(outDir string) error {
	inputFile := "/var/log/vmlog/vmlog.LATEST"
	err := fsutil.CopyFile(inputFile, filepath.Join(outDir, "vmlog.LATEST"))
	if err != nil {
		return errors.Wrap(err, "cannot copy vmlog.LATEST")
	}
	return nil
}

// getMemdLogs copies the memd log files into the output directory of the test.
func getMemdLogs(outDir string) error {
	inputDir := "/var/log/memd"
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		return errors.Wrap(err, "cannot read memd")
	}

	for _, file := range files {
		inputFile := filepath.Join(inputDir, file.Name())
		err = fsutil.CopyFile(inputFile, filepath.Join(outDir, file.Name()))
		if err != nil {
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
	clipFilesPattern := "/var/log/memd/memd.clip*.log"
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
	testEnv := TestEnv{}
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	testEnv.Chrome = cr

	a, err := arc.New(ctx, outDir)
	if err != nil {
		cr.Close(ctx)
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	testEnv.ARC = a

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		cr.Close(ctx)
		a.Close()
		return nil, errors.Wrap(err, "failed initializing UI Automator")
	}
	testEnv.ARCDevice = d
	tconn, err := testEnv.Chrome.TestAPIConn(ctx)
	if err != nil {
		cr.Close(ctx)
		a.Close()
		d.Close()
		return nil, errors.Wrap(err, "creating test API connection failed")
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		cr.Close(ctx)
		a.Close()
		d.Close()
		return nil, errors.Wrap(err, "failed to enable Crostini preference setting")
	}
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		cr.Close(ctx)
		a.Close()
		d.Close()
		return nil, errors.Wrap(err, "failed to set up component")
	}

	conc, err := vm.NewConcierge(ctx, testEnv.Chrome.User())
	if err != nil {
		cr.Close(ctx)
		a.Close()
		d.Close()
		return nil, errors.Wrap(err, "failed to start concierge")
	}

	testvm := vm.NewDefaultVM(conc)
	err = testvm.Start(ctx)
	if err != nil {
		cr.Close(ctx)
		a.Close()
		d.Close()
		return nil, errors.Wrap(err, "failed to start VM")
	}
	testEnv.VM = testvm
	return &testEnv, nil

}

// Close closes the Chrome, ARC, ARC UI Automator device, and VM instances used in the TestEnv.
func (testEnv *TestEnv) Close(ctx context.Context) {
	testEnv.Chrome.Close(ctx)
	testEnv.ARC.Close()
	testEnv.ARCDevice.Close()
	testEnv.VM.Stop(ctx)
}

// RunMemoryUserTest creates a new TestEnv and then runs ARC, Chrome, and VM tasks in parallel.
// It also logs memory and cpu usage throughout the test, and copies output from /var/log/memd and /var/log/vmlog
// when finished.
func RunMemoryUserTest(ctx context.Context, s *testing.State, memoryTasks []MemoryTask) {
	testEnv, err := newTestEnv(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed creating the test environment: ", err)
	}
	defer testEnv.Close(ctx)

	err = prepareMemdLogging(ctx)
	if err != nil {
		s.Error("Failed to prepare memd logging: ", err)
	}
	go func() {
		err := logUse(ctx, "cat", "/proc/meminfo", filepath.Join(s.OutDir(), "memory_use.txt"))
		if err != nil {
			s.Error("Failed to log memory use: ", err)
		}
	}()
	go func() {
		err = logUse(ctx, "iostat", "-c", filepath.Join(s.OutDir(), "cpu_use.txt"))
		if err != nil {
			s.Error("Failed to log cpu use: ", err)
		}
	}()

	var wg sync.WaitGroup

	for _, task := range memoryTasks {
		wg.Add(1)
		go func(task MemoryTask) {
			defer wg.Done()
			err := task.Run(ctx, testEnv)
			if err != nil {
				s.Error("Failed to run memory task: ", err)
			}
		}(task)
		defer task.Close(ctx, testEnv)
	}

	wg.Wait()

	err = getVMLog(s.OutDir())
	if err != nil {
		s.Error("Failed to get vmlog file: ", err)
	}
	err = getMemdLogs(s.OutDir())
	if err != nil {
		s.Error("Failed to get memd files: ", err)
	}

}
