// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/vm/common"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	kB int64 = 1024
	mB       = 1024 * kB
	gB       = 1024 * mB

	runBlogbench string = "run-blogbench.sh"
)

type config struct {
	kind         string
	externalDisk bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Blogbench,
		Desc:         "Tests crosvm storage device small file performance",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Data:         []string{common.VirtiofsKernel(), runBlogbench},
		SoftwareDeps: []string{"vm_host"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "block",
				Val:  config{kind: "block"},
			},
			{
				Name: "block_external",
				Val:  config{kind: "block", externalDisk: true},
			},
			{
				Name: "block_btrfs",
				Val:  config{kind: "block_btrfs"},
			},
			{
				Name: "block_btrfs_external",
				Val:  config{kind: "block_btrfs", externalDisk: true},
			},
			{
				Name: "virtiofs",
				Val:  config{kind: "fs"},
			},
			{
				Name: "virtiofs_external",
				Val:  config{kind: "fs", externalDisk: true},
			},
			{
				Name: "virtiofs_dax",
				Val:  config{kind: "fs_dax"},
			},
			{
				Name: "virtiofs_dax_external",
				Val:  config{kind: "fs_dax", externalDisk: true},
			},
			{
				Name: "p9",
				Val:  config{kind: "p9"},
			},
			{
				Name: "direct_mount",
				Val:  config{kind: "direct"},
			},
		},
	})
}

func Blogbench(ctx context.Context, s *testing.State) {
	config := s.Param().(config)
	// Create a temporary directory on external disk or stateful partition rather than in memory.
	var td string
	if config.externalDisk {
		td = "/media/removable/USBDrive/tast.vm.Blogbench"
		if err := os.Mkdir(td, 0755); err != nil {
			s.Fatal("Failed to create shared directory: ", err)
		}
	} else {
		var err error
		td, err = ioutil.TempDir("/usr/local/tmp", "tast.vm.Blogbench.")
		if err != nil {
			s.Fatal("Failed to create temporary directory: ", err)
		}
	}
	defer os.RemoveAll(td)

	/*
		vmlinux := s.DataPath(common.VirtiofsKernel())

		kernel := filepath.Join(td, "kernel")
		if err := common.UnpackKernel(ctx, vmlinux, kernel); err != nil {
			s.Fatal("Failed to unpack kernel: ", err)
		}
	*/
	// Use a custom kernel with unmerged virtio-fs patches.
	kernel := "/mnt/stateful_partition/virtiofs_dax_kernel"

	shared := filepath.Join(td, "shared")
	if err := os.Mkdir(shared, 0755); err != nil {
		s.Fatal("Failed to create shared directory: ", err)
	}

	block := filepath.Join(td, "block")
	f, err := os.Create(block)
	if err != nil {
		s.Fatal("Failed to create block device file: ", err)
	}

	if err := f.Truncate(8 * gB); err != nil {
		s.Fatal("Failed to set block device file size: ", err)
	}
	f.Close()

	logFile := filepath.Join(s.OutDir(), "serial.log")

	// Increase the max open file limit as the benchmark creates a lot of files.
	args := []string{
		"--nofile=262144",
		"crosvm", "run",
		"-c", "1",
		"-m", "1024",
		"-s", td,
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
	}

	kind := config.kind
	var tag string
	if kind == "block" || kind == "block_btrfs" {
		tag = "/dev/vda"
		args = append(args, "--rwdisk", block)
	} else if kind == "fs" || kind == "fs_dax" {
		tag = "shared"
		args = append(args, "--shared-dir",
			fmt.Sprintf("%s:%s:type=%s:cache=always:timeout=3600:writeback=true", shared, tag, "fs"))
	} else if kind == "p9" {
		tag = "shared"
		args = append(args, "--shared-dir", fmt.Sprintf("%s:%s", shared, tag))
	} else if kind == "direct" {
		// Assume /dev/sda is mounted at /media/removable/USBDrive/,
		// We unmount /dev/sda in the host and pass it to crosvm.
		if err := syscall.Unmount("/media/removable/USBDrive/", 0); err != nil {
			s.Fatal("Failed to unmount USB drive: ", err)
		}
		defer func() {
			if err := syscall.Mount("/dev/sda", "/media/removable/USBDrive/", "ext4", 0, ""); err != nil {
				s.Log("Failed to remount USB drive: ", err)
			}
		}()

		tag = "/dev/vda"
		args = append(args, "--rwdisk", "/dev/sda")
	} else {
		s.Fatal("Unknown storage device type: ", err)
	}

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runBlogbench)),
		"--",
		kind,
		tag,
		td,
	}

	args = append(args, "-p", strings.Join(params, " "), kernel)

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	s.Log("args: ", args)
	s.Log("Running blogbench")
	cmd := testexec.CommandContext(ctx, "prlimit", args...)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	log, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Fatal("Failed to read serial log: ", err)
	}

	lines := strings.Split(string(log), "\n")

	// The messages we care about are at the end of the log so iterate over the lines in
	// reverse order.
	writeScore := 0
	readScore := 0
	for idx := len(lines) - 1; idx >= 0; idx-- {
		if writeScore != 0 && readScore != 0 {
			break
		}

		if !strings.HasPrefix(lines[idx], "Final score for") {
			continue
		}

		if _, err := fmt.Sscanf(lines[idx], "Final score for writes:\t%v", &writeScore); err == nil {
			continue
		}

		if _, err := fmt.Sscanf(lines[idx], "Final score for reads :\t%v", &readScore); err != nil {
			s.Error("Failed to get score: ", err)
		}
	}

	s.Logf("Read score = %v, write score = %v", readScore, writeScore)

	perfValues := perf.NewValues()
	perfValues.Append(perf.Metric{
		Name:      "read",
		Variant:   kind,
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, float64(readScore))

	perfValues.Append(perf.Metric{
		Name:      "write",
		Variant:   kind,
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, float64(writeScore))
	perfValues.Save(s.OutDir())
}
