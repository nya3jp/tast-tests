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
	"time"

	"chromiumos/tast/local/bundles/cros/vm/common"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const runFio string = "run-fio.sh"

type param struct {
	kind string
	job  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fio,
		Desc:         "Tests crosvm storage device bandwidth",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Data:         []string{common.VirtiofsKernel(), runFio},
		SoftwareDeps: []string{"vm_host"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{
			testing.Param{
				Name:      "block_boot",
				ExtraData: []string{"fio_boot.job"},
				Val: param{
					kind: "block",
					job:  "fio_boot.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_boot",
				ExtraData: []string{"fio_boot.job"},
				Val: param{
					kind: "fs",
					job:  "fio_boot.job",
				},
			},
			testing.Param{
				Name:      "p9_boot",
				ExtraData: []string{"fio_boot.job"},
				Val: param{
					kind: "p9",
					job:  "fio_boot.job",
				},
			},
			testing.Param{
				Name:      "block_login",
				ExtraData: []string{"fio_login.job"},
				Val: param{
					kind: "block",
					job:  "fio_login.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_login",
				ExtraData: []string{"fio_login.job"},
				Val: param{
					kind: "fs",
					job:  "fio_login.job",
				},
			},
			testing.Param{
				Name:      "p9_login",
				ExtraData: []string{"fio_login.job"},
				Val: param{
					kind: "p9",
					job:  "fio_login.job",
				},
			},
			testing.Param{
				Name:      "block_surfing",
				ExtraData: []string{"fio_surfing.job"},
				Val: param{
					kind: "block",
					job:  "fio_surfing.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_surfing",
				ExtraData: []string{"fio_surfing.job"},
				Val: param{
					kind: "fs",
					job:  "fio_surfing.job",
				},
			},
			testing.Param{
				Name:      "p9_surfing",
				ExtraData: []string{"fio_surfing.job"},
				Val: param{
					kind: "p9",
					job:  "fio_surfing.job",
				},
			},
			testing.Param{
				Name:      "block_randread",
				ExtraData: []string{"fio_rand_read.job"},
				Val: param{
					kind: "block",
					job:  "fio_rand_read.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_randread",
				ExtraData: []string{"fio_rand_read.job"},
				Val: param{
					kind: "fs",
					job:  "fio_rand_read.job",
				},
			},
			testing.Param{
				Name:      "p9_randread",
				ExtraData: []string{"fio_rand_read.job"},
				Val: param{
					kind: "p9",
					job:  "fio_rand_read.job",
				},
			},
			testing.Param{
				Name:      "block_seqread",
				ExtraData: []string{"fio_seq_read.job"},
				Val: param{
					kind: "block",
					job:  "fio_seq_read.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_seqread",
				ExtraData: []string{"fio_seq_read.job"},
				Val: param{
					kind: "fs",
					job:  "fio_seq_read.job",
				},
			},
			testing.Param{
				Name:      "p9_seqread",
				ExtraData: []string{"fio_seq_read.job"},
				Val: param{
					kind: "p9",
					job:  "fio_seq_read.job",
				},
			},
			testing.Param{
				Name:      "block_seqwrite",
				ExtraData: []string{"fio_seq_write.job"},
				Val: param{
					kind: "block",
					job:  "fio_seq_write.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_seqwrite",
				ExtraData: []string{"fio_seq_write.job"},
				Val: param{
					kind: "fs",
					job:  "fio_seq_write.job",
				},
			},
			testing.Param{
				Name:      "p9_seqwrite",
				ExtraData: []string{"fio_seq_write.job"},
				Val: param{
					kind: "p9",
					job:  "fio_seq_write.job",
				},
			},
			testing.Param{
				Name:      "block_randwrite",
				ExtraData: []string{"fio_rand_write.job"},
				Val: param{
					kind: "block",
					job:  "fio_rand_write.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_randwrite",
				ExtraData: []string{"fio_rand_write.job"},
				Val: param{
					kind: "fs",
					job:  "fio_rand_write.job",
				},
			},
			testing.Param{
				Name:      "p9_randwrite",
				ExtraData: []string{"fio_rand_write.job"},
				Val: param{
					kind: "p9",
					job:  "fio_rand_write.job",
				},
			},
			testing.Param{
				Name:      "block_stress_rw",
				ExtraData: []string{"fio_stress_rw.job"},
				Val: param{
					kind: "block",
					job:  "fio_stress_rw.job",
				},
			},
			testing.Param{
				Name:      "virtiofs_stress_rw",
				ExtraData: []string{"fio_stress_rw.job"},
				Val: param{
					kind: "fs",
					job:  "fio_stress_rw.job",
				},
			},
			testing.Param{
				Name:      "p9_stress_rw",
				ExtraData: []string{"fio_stress_rw.job"},
				Val: param{
					kind: "p9",
					job:  "fio_stress_rw.job",
				},
			},
		},
	})
}

func Fio(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.Fio.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	vmlinux := s.DataPath(common.VirtiofsKernel())

	kernel := filepath.Join(td, "kernel")
	if err := common.UnpackKernel(ctx, vmlinux, kernel); err != nil {
		s.Fatal("Failed to unpack kernel: ", err)
	}

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
		"-m", "512",
		"-s", td,
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
	}

	p := s.Param().(param)
	kind := p.kind
	job := p.job

	var tag string
	if kind == "block" {
		tag = "/dev/vda"
		args = append(args, "--rwdisk", block)
	} else if kind == "fs" {
		tag = "shared"
		args = append(args, "--shared-dir",
			fmt.Sprintf("%s:%s:type=%s:cache=always:timeout=3600:writeback=true", shared, tag, kind))
	} else if kind == "p9" {
		tag = "shared"
		args = append(args, "--shared-dir", fmt.Sprintf("%s:%s", shared, tag))
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
