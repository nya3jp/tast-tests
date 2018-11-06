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

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// MemoryTask is an interface with the method RunMemoryTask which takes a Chrome instance
// This allows various types of activities, such as ARC, Chrome, and VMs, to be defined
// using the same Chrome instance and run in parallel.
type MemoryTask interface {
	RunMemoryTask(ctx context.Context, s *testing.State, cr *chrome.Chrome)
}

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
		default:
			b, err := exec.Command("cat", "/proc/meminfo").CombinedOutput()
			if err != nil {
				s.Error("cat /proc/meminfo failed: ", err)
			}
			if _, err := file.Write(b); err != nil {
				s.Error("Writing memory use output file failed: ", err)
			}
			if _, err := file.Write([]byte("\n")); err != nil {
				s.Error("Writing memory use output file failed: ", err)
			}
			time.After(5 * time.Second)
		}
	}
}

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
			b, err := exec.Command("iostat", "-c").CombinedOutput()
			if err != nil {
				s.Error("iostat -c failed: ", err)
			}
			if _, err := file.Write(b); err != nil {
				s.Error("Writing cpu use output file failed: ", err)
			}
		}
	}
}

func getVmlog(s *testing.State) {
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

func getMemdLogs(s *testing.State) {
	inputDir := "/var/log/memd"
	//outputDir := filepath.Join(s.OutDir(), "memd")
	//err := os.Mkdir(outputDir, 0664)
	//if err != nil {
	//	s.Error("Cannot create output dir: ", err)
	//}
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

func startMemdLogging(ctx context.Context, s *testing.State) {
	defer func() {
		// Restart memd
		if err := upstart.RestartJob(ctx, "memd"); err != nil {
			s.Error("Cannot restart memd: ", err)
		}
	}()

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

	go testexec.CommandContext(ctx, "memd", "always-poll-fast")

}

// RunMemoryUserTest starts a Chrome instance and then runs ARC, Chrome, and VM tasks.
// It also logs memory and cpu usage throughout the test, and copies output from /var/log/memd and /var/log/vmlog
// when finished.
func RunMemoryUserTest(ctx context.Context, s *testing.State, aTask AndroidTask, cTask ChromeTask, vmTask VMTask) {
	stopMemLog := make(chan int)
	go logMemoryUse(s, stopMemLog)
	stopCPULog := make(chan int)
	go logCPUUse(s, stopCPULog)
	startMemdLogging(ctx, s)

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	aTask.RunMemoryTask(ctx, s, cr)

	var wg sync.WaitGroup
	var tasks = []MemoryTask{cTask, vmTask}

	for _, task := range tasks {
		wg.Add(1)
		go func(task MemoryTask) {
			defer wg.Done()
			task.RunMemoryTask(ctx, s, cr)
		}(task)
	}

	wg.Wait()

	stopMemLog <- 0
	stopCPULog <- 0
	getVmlog(s)
	getMemdLogs(s)

}
