// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs
package memoryuser

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// TestEnvironment is a struct containing the Chrome Instance, ARC instance,
// ARC UI Automator device, and VM to be used across the test.
type TestEnvironment struct {
	Chrome    *chrome.Chrome
	Arc       *arc.ARC
	ArcDevice *ui.Device
	VM        *vm.VM
}

// MemoryTask is an interface with the method RunMemoryTask which takes a TestEnvironment
// This allows various types of activities, such as ARC, Chrome, and VMs, to be defined
// using the same setup and run in parallel.
type MemoryTask interface {
	RunMemoryTask(ctx context.Context, s *testing.State, testEnv *TestEnvironment)
}

// logMemoryUse logs the output of "cat /proc/meminfo" into a file every 5 seconds
func logMemoryUse(s *testing.State, stop chan int) {
	file, err := os.OpenFile(filepath.Join(s.OutDir(), "memory_use.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Error("Failed to create log file for memory use: ", err)
	}
	for {
		select {
		case <-stop:
			if err := file.Close(); err != nil {
				s.Error("Failed to close memory use file: ", err)
			}
			return
		case <-time.After(5 * time.Second):
			b, err := exec.Command("date").CombinedOutput()
			if err != nil {
				s.Error("date failed: ", err)
			}
			if _, err := file.Write(b); err != nil {
				s.Error("Writing memory use output file failed: ", err)
			}

			b, err = exec.Command("cat", "/proc/meminfo").CombinedOutput()
			if err != nil {
				s.Error("cat /proc/meminfo failed: ", err)
			}
			if _, err := file.Write(b); err != nil {
				s.Error("Writing memory use output file failed: ", err)
			}
		}
	}
}

// logCPUUse logs the output of "iostat -c" into a file every 5 seconds
func logCPUUse(s *testing.State, stop chan int) {
	file, err := os.OpenFile(filepath.Join(s.OutDir(), "cpu_use.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Error("Failed to create log file for cpu use: ", err)
	}
	for {
		select {
		case <-stop:
			if err := file.Close(); err != nil {
				s.Error("Failed to close cpu use file: ", err)
			}
			return
		case <-time.After(5 * time.Second):
			b, err := exec.Command("date").CombinedOutput()
			if err != nil {
				s.Error("date failed: ", err)
			}
			if _, err := file.Write(b); err != nil {
				s.Error("Writing memory use output file failed: ", err)
			}

			b, err = exec.Command("iostat", "-c").CombinedOutput()
			if err != nil {
				s.Error("iostat -c failed: ", err)
			}
			if _, err := file.Write(b); err != nil {
				s.Error("Writing cpu use output file failed: ", err)
			}
		}
	}
}

// getVMLog copies the file vmlog.LATEST to the output directory of the test
func getVMLog(s *testing.State) {
	inputFile := "/var/log/vmlog/vmlog.LATEST"
	input, err := ioutil.ReadFile(inputFile)
	if err != nil {
		s.Error("Cannot read vmlog: ", err)
	}
	err = ioutil.WriteFile(filepath.Join(s.OutDir(), "vmlog.LATEST"), input, 0644)
	if err != nil {
		s.Error("Cannot copy vmlog.LATEST: ", err)
	}
}

// getMemdLogs copies the memd log files into the output directory of the test
func getMemdLogs(s *testing.State) {
	inputDir := "/var/log/memd"
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		s.Error("Cannot read memd: ", err)
	}

	for _, file := range files {
		inputFile := filepath.Join(inputDir, file.Name())
		input, err := ioutil.ReadFile(inputFile)
		if err != nil {
			s.Error("Cannot read memd file: ", err)
		}
		err = ioutil.WriteFile(filepath.Join(s.OutDir(), file.Name()), input, 0644)
		if err != nil {
			s.Error("Cannot copy memd file: ", err)
		}
	}
}

// prepareMemdLogging restarts memd and removes old memd log files
func prepareMemdLogging(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "memd"); err != nil {
		s.Error("Cannot restart memd: ", err)
	}
	clipFilesPattern := "/var/log/memd/memd.clip*.log"
	// Remove any clip files from /var/log/memd.
	files, err := filepath.Glob(clipFilesPattern)
	if err != nil {
		s.Fatalf("Cannot list %v: %v", clipFilesPattern, err)
	}
	for _, file := range files {
		if err = os.Remove(file); err != nil {
			s.Fatalf("Cannot remove %v: %v", file, err)
		}
	}

}

// setupChrome starts a Chrome instance to use in the TestEnvironment
func setupChrome(ctx context.Context, s *testing.State, testEnv *TestEnvironment) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	testEnv.Chrome = cr
}

// setupARC starts a new ARC instance and UI Automator device to use in the TestEnvironment
func setupARC(ctx context.Context, s *testing.State, testEnv *TestEnvironment) {
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	testEnv.Arc = a

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	testEnv.ArcDevice = d
}

// setupVM starts a new default VM to use in the TestEnvironment
func setupVM(ctx context.Context, s *testing.State, testEnv *TestEnvironment) {
	s.Log("Enabling Crostini preference setting")
	tconn, err := testEnv.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Restarting Concierge")
	conc, err := vm.NewConcierge(ctx, testEnv.Chrome.User())
	if err != nil {
		s.Fatal("Failed to start concierge: ", err)
	}

	testvm := vm.NewDefaultVM(conc)
	err = testvm.Start(ctx)
	if err != nil {
		s.Fatal("Failed to start VM: ", err)
	}
	testEnv.VM = testvm

}

// RunMemoryUserTest sets up the TestEnvironment and then runs ARC, Chrome, and VM tasks in parallel.
// It also logs memory and cpu usage throughout the test, and copies output from /var/log/memd and /var/log/vmlog
// when finished.
func RunMemoryUserTest(ctx context.Context, s *testing.State, memoryTasks []MemoryTask) {
	stopMemLog := make(chan int)
	stopCPULog := make(chan int)
	go logMemoryUse(s, stopMemLog)
	go logCPUUse(s, stopCPULog)
	prepareMemdLogging(ctx, s)

	testEnv := TestEnvironment{}
	setupChrome(ctx, s, &testEnv)
	setupARC(ctx, s, &testEnv)
	setupVM(ctx, s, &testEnv)
	defer testEnv.Chrome.Close(ctx)
	defer testEnv.Arc.Close()
	defer testEnv.ArcDevice.Close()
	defer testEnv.VM.Stop(ctx)

	var wg sync.WaitGroup

	for _, task := range memoryTasks {
		wg.Add(1)
		go func(task MemoryTask) {
			defer wg.Done()
			task.RunMemoryTask(ctx, s, &testEnv)
		}(task)
	}

	wg.Wait()

	stopMemLog <- 0
	stopCPULog <- 0
	getVMLog(s)
	getMemdLogs(s)

}
