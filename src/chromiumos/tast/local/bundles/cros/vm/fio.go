// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"encoding/json"
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

const runFio string = "run-fio.sh"

type param struct {
	kind         string
	job          string
	externalDisk bool
}

func init() {

	jobs := []string{"boot", "login", "surfing", "seq_read", "rand_read", "seq_write", "rand_write", "stress_rw"}

	// I know it's not allowed to generate parameters dynamically, but let me do so only for testing.
	var params []testing.Param
	for _, j := range jobs {
		job := fmt.Sprintf("fio_%s.job", j)

		params = append(params,
			testing.Param{
				Name:      fmt.Sprintf("block_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind: "block",
					job:  job,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("block_external_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind:         "block",
					job:          job,
					externalDisk: true,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("block_btrfs_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind: "block_btrfs",
					job:  job,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("block_btrfs_external_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind:         "block_btrfs",
					job:          job,
					externalDisk: true,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("virtiofs_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind: "fs",
					job:  job,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("virtiofs_external_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind:         "fs",
					job:          job,
					externalDisk: true,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("virtiofs_dax_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind: "fs_dax",
					job:  job,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("virtiofs_dax_external_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind:         "fs_dax",
					job:          job,
					externalDisk: true,
				},
			},
			testing.Param{
				Name:      fmt.Sprintf("direct_mount_%s", j),
				ExtraData: []string{job},
				Val: param{
					kind: "direct",
					job:  job,
				},
			},
		)
	}

	testing.AddTest(&testing.Test{
		Func:         Fio,
		Desc:         "Tests crosvm storage device bandwidth",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Data:         []string{common.VirtiofsKernel(), runFio},
		SoftwareDeps: []string{"vm_host"},
		Timeout:      15 * time.Minute,
		Params:       params,
	})
}

func Fio(ctx context.Context, s *testing.State) {
	p := s.Param().(param)

	// Create a temporary directory on the stateful partition rather than in memory.
	var td string
	if p.externalDisk {
		td = "/media/removable/USBDrive/tast.vm.Fio"
		if err := os.Mkdir(td, 0755); err != nil {
			s.Fatal("Failed to create shared directory: ", err)
		}
	} else {
		var err error
		td, err = ioutil.TempDir("/usr/local/tmp", "tast.vm.Fio.")
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
	defer f.Close()

	if err := f.Truncate(8 * 1024 * 1024 * 1024); err != nil {
		s.Fatal("Failed to set block device file size: ", err)
	}

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

	kind := p.kind
	job := p.job

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

	fioOutput := filepath.Join(s.OutDir(), "fio-output.json")

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runFio)),
		"--",
		kind,
		tag,
		td,
		fioOutput,
		s.DataPath(job),
	}

	args = append(args, "-p", strings.Join(params, " "), kernel)

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	s.Log("Running fio")
	cmd := testexec.CommandContext(ctx, "prlimit", args...)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	buf, err := ioutil.ReadFile(fioOutput)
	if err != nil {
		s.Fatal("Failed to read fio results: ", err)
	}

	results := make(map[string]interface{})
	if err := json.Unmarshal(buf, &results); err != nil {
		s.Fatal("Failed to unmarshal fio results: ", err)
	}

	perfValues := perf.NewValues()
	jobs := results["jobs"].([]interface{})
	for _, i := range jobs {
		j := i.(map[string]interface{})
		read := j["read"].(map[string]interface{})
		readBW := read["bw"].(float64)
		perfValues.Append(perf.Metric{
			Name:      "read",
			Variant:   j["jobname"].(string),
			Unit:      "KiBps",
			Direction: perf.BiggerIsBetter,
			Multiple:  true,
		}, readBW)

		write := j["write"].(map[string]interface{})
		writeBW := write["bw"].(float64)
		perfValues.Append(perf.Metric{
			Name:      "write",
			Variant:   j["jobname"].(string),
			Unit:      "KiBps",
			Direction: perf.BiggerIsBetter,
			Multiple:  true,
		}, writeBW)

		s.Logf("Jobname = %s, read bw = %v, write bw = %v", j["jobname"], readBW, writeBW)
	}
	perfValues.Save(s.OutDir())
}
