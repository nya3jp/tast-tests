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

func fioJobs() []string {
	return []string{
		"boot.job",
		"login.job",
		"surfing.job",
		"randread-libaio.job",
		"seqread-libaio.job",
		"randread-psync-multi.job",
		"seqread-mmap-multi.job",
		"randread-psync.job",
		"seqwrite-mmap.job",
		"randwrite-mmap.job",
		"seqwrite-libaio-multi.job",
		"randwrite-psync-multi.job",
		"seqread-libaio-multi.job",
		"randread-mmap.job",
		"seqwrite-libaio.job",
		"seqwrite-mmap-multi.job",
		"randread-libaio-multi.job",
		"randwrite-mmap-multi.job",
		"seqread-mmap.job",
		"seqread-psync.job",
		"randwrite-psync.job",
		"randread-mmap-multi.job",
		"seqwrite-psync-multi.job",
		"randwrite-libaio-multi.job",
		"randwrite-libaio.job",
		"seqwrite-psync.job",
		"seqread-psync-multi.job",
	}
}

// We could use a nice function to generate the parameters or we could have upload hooks that
// insist that the Params field must be a []string literal.
// func getParams() []testing.Param {
// 	var out []testing.Param
// 	for _, job := range fioJobs() {
// 		jobName := strings.ReplaceAll(strings.TrimSuffix(job, path.Ext(job)), "-", "_")
// 		out = append(out,
// 			testing.Param{
// 				Name: fmt.Sprintf("block_%s", jobName),
// 				Val: param{
// 					kind: "block",
// 					job:  job,
// 				},
// 			},
// 			testing.Param{
// 				Name: fmt.Sprintf("virtiofs_%s", jobName),
// 				Val: param{
// 					kind: "fs",
// 					job:  job,
// 				},
// 			},
// 			testing.Param{
// 				Name: fmt.Sprintf("p9_%s", jobName),
// 				Val: param{
// 					kind: "p9",
// 					job:  job,
// 				},
// 			},
// 		)
// 	}

// 	return out
// }

func init() {
	testing.AddTest(&testing.Test{
		Func:         Fio,
		Desc:         "Tests crosvm storage device bandwidth",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Data:         append(fioJobs(), common.VirtiofsKernel(), runFio),
		SoftwareDeps: []string{"vm_host"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{
			testing.Param{
				Name: "block_boot",
				Val: param{
					kind: "block",
					job:  "boot.job",
				},
			},
			testing.Param{
				Name: "virtiofs_boot",
				Val: param{
					kind: "fs",
					job:  "boot.job",
				},
			},
			testing.Param{
				Name: "p9_boot",
				Val: param{
					kind: "p9",
					job:  "boot.job",
				},
			},
			testing.Param{
				Name: "block_login",
				Val: param{
					kind: "block",
					job:  "login.job",
				},
			},
			testing.Param{
				Name: "virtiofs_login",
				Val: param{
					kind: "fs",
					job:  "login.job",
				},
			},
			testing.Param{
				Name: "p9_login",
				Val: param{
					kind: "p9",
					job:  "login.job",
				},
			},
			testing.Param{
				Name: "block_surfing",
				Val: param{
					kind: "block",
					job:  "surfing.job",
				},
			},
			testing.Param{
				Name: "virtiofs_surfing",
				Val: param{
					kind: "fs",
					job:  "surfing.job",
				},
			},
			testing.Param{
				Name: "p9_surfing",
				Val: param{
					kind: "p9",
					job:  "surfing.job",
				},
			},
			testing.Param{
				Name: "block_randread_libaio",
				Val: param{
					kind: "block",
					job:  "randread-libaio.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randread_libaio",
				Val: param{
					kind: "fs",
					job:  "randread-libaio.job",
				},
			},
			testing.Param{
				Name: "p9_randread_libaio",
				Val: param{
					kind: "p9",
					job:  "randread-libaio.job",
				},
			},
			testing.Param{
				Name: "block_seqread_libaio",
				Val: param{
					kind: "block",
					job:  "seqread-libaio.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqread_libaio",
				Val: param{
					kind: "fs",
					job:  "seqread-libaio.job",
				},
			},
			testing.Param{
				Name: "p9_seqread_libaio",
				Val: param{
					kind: "p9",
					job:  "seqread-libaio.job",
				},
			},
			testing.Param{
				Name: "block_randread_psync_multi",
				Val: param{
					kind: "block",
					job:  "randread-psync-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randread_psync_multi",
				Val: param{
					kind: "fs",
					job:  "randread-psync-multi.job",
				},
			},
			testing.Param{
				Name: "p9_randread_psync_multi",
				Val: param{
					kind: "p9",
					job:  "randread-psync-multi.job",
				},
			},
			testing.Param{
				Name: "block_seqread_mmap_multi",
				Val: param{
					kind: "block",
					job:  "seqread-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqread_mmap_multi",
				Val: param{
					kind: "fs",
					job:  "seqread-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "p9_seqread_mmap_multi",
				Val: param{
					kind: "p9",
					job:  "seqread-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "block_randread_psync",
				Val: param{
					kind: "block",
					job:  "randread-psync.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randread_psync",
				Val: param{
					kind: "fs",
					job:  "randread-psync.job",
				},
			},
			testing.Param{
				Name: "p9_randread_psync",
				Val: param{
					kind: "p9",
					job:  "randread-psync.job",
				},
			},
			testing.Param{
				Name: "block_seqwrite_mmap",
				Val: param{
					kind: "block",
					job:  "seqwrite-mmap.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqwrite_mmap",
				Val: param{
					kind: "fs",
					job:  "seqwrite-mmap.job",
				},
			},
			testing.Param{
				Name: "p9_seqwrite_mmap",
				Val: param{
					kind: "p9",
					job:  "seqwrite-mmap.job",
				},
			},
			testing.Param{
				Name: "block_randwrite_mmap",
				Val: param{
					kind: "block",
					job:  "randwrite-mmap.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randwrite_mmap",
				Val: param{
					kind: "fs",
					job:  "randwrite-mmap.job",
				},
			},
			testing.Param{
				Name: "p9_randwrite_mmap",
				Val: param{
					kind: "p9",
					job:  "randwrite-mmap.job",
				},
			},
			testing.Param{
				Name: "block_seqwrite_libaio_multi",
				Val: param{
					kind: "block",
					job:  "seqwrite-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqwrite_libaio_multi",
				Val: param{
					kind: "fs",
					job:  "seqwrite-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "p9_seqwrite_libaio_multi",
				Val: param{
					kind: "p9",
					job:  "seqwrite-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "block_randwrite_psync_multi",
				Val: param{
					kind: "block",
					job:  "randwrite-psync-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randwrite_psync_multi",
				Val: param{
					kind: "fs",
					job:  "randwrite-psync-multi.job",
				},
			},
			testing.Param{
				Name: "p9_randwrite_psync_multi",
				Val: param{
					kind: "p9",
					job:  "randwrite-psync-multi.job",
				},
			},
			testing.Param{
				Name: "block_seqread_libaio_multi",
				Val: param{
					kind: "block",
					job:  "seqread-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqread_libaio_multi",
				Val: param{
					kind: "fs",
					job:  "seqread-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "p9_seqread_libaio_multi",
				Val: param{
					kind: "p9",
					job:  "seqread-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "block_randread_mmap",
				Val: param{
					kind: "block",
					job:  "randread-mmap.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randread_mmap",
				Val: param{
					kind: "fs",
					job:  "randread-mmap.job",
				},
			},
			testing.Param{
				Name: "p9_randread_mmap",
				Val: param{
					kind: "p9",
					job:  "randread-mmap.job",
				},
			},
			testing.Param{
				Name: "block_seqwrite_libaio",
				Val: param{
					kind: "block",
					job:  "seqwrite-libaio.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqwrite_libaio",
				Val: param{
					kind: "fs",
					job:  "seqwrite-libaio.job",
				},
			},
			testing.Param{
				Name: "p9_seqwrite_libaio",
				Val: param{
					kind: "p9",
					job:  "seqwrite-libaio.job",
				},
			},
			testing.Param{
				Name: "block_seqwrite_mmap_multi",
				Val: param{
					kind: "block",
					job:  "seqwrite-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqwrite_mmap_multi",
				Val: param{
					kind: "fs",
					job:  "seqwrite-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "p9_seqwrite_mmap_multi",
				Val: param{
					kind: "p9",
					job:  "seqwrite-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "block_randread_libaio_multi",
				Val: param{
					kind: "block",
					job:  "randread-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randread_libaio_multi",
				Val: param{
					kind: "fs",
					job:  "randread-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "p9_randread_libaio_multi",
				Val: param{
					kind: "p9",
					job:  "randread-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "block_randwrite_mmap_multi",
				Val: param{
					kind: "block",
					job:  "randwrite-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randwrite_mmap_multi",
				Val: param{
					kind: "fs",
					job:  "randwrite-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "p9_randwrite_mmap_multi",
				Val: param{
					kind: "p9",
					job:  "randwrite-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "block_seqread_mmap",
				Val: param{
					kind: "block",
					job:  "seqread-mmap.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqread_mmap",
				Val: param{
					kind: "fs",
					job:  "seqread-mmap.job",
				},
			},
			testing.Param{
				Name: "p9_seqread_mmap",
				Val: param{
					kind: "p9",
					job:  "seqread-mmap.job",
				},
			},
			testing.Param{
				Name: "block_seqread_psync",
				Val: param{
					kind: "block",
					job:  "seqread-psync.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqread_psync",
				Val: param{
					kind: "fs",
					job:  "seqread-psync.job",
				},
			},
			testing.Param{
				Name: "p9_seqread_psync",
				Val: param{
					kind: "p9",
					job:  "seqread-psync.job",
				},
			},
			testing.Param{
				Name: "block_randwrite_psync",
				Val: param{
					kind: "block",
					job:  "randwrite-psync.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randwrite_psync",
				Val: param{
					kind: "fs",
					job:  "randwrite-psync.job",
				},
			},
			testing.Param{
				Name: "p9_randwrite_psync",
				Val: param{
					kind: "p9",
					job:  "randwrite-psync.job",
				},
			},
			testing.Param{
				Name: "block_randread_mmap_multi",
				Val: param{
					kind: "block",
					job:  "randread-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randread_mmap_multi",
				Val: param{
					kind: "fs",
					job:  "randread-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "p9_randread_mmap_multi",
				Val: param{
					kind: "p9",
					job:  "randread-mmap-multi.job",
				},
			},
			testing.Param{
				Name: "block_seqwrite_psync_multi",
				Val: param{
					kind: "block",
					job:  "seqwrite-psync-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqwrite_psync_multi",
				Val: param{
					kind: "fs",
					job:  "seqwrite-psync-multi.job",
				},
			},
			testing.Param{
				Name: "p9_seqwrite_psync_multi",
				Val: param{
					kind: "p9",
					job:  "seqwrite-psync-multi.job",
				},
			},
			testing.Param{
				Name: "block_randwrite_libaio_multi",
				Val: param{
					kind: "block",
					job:  "randwrite-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randwrite_libaio_multi",
				Val: param{
					kind: "fs",
					job:  "randwrite-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "p9_randwrite_libaio_multi",
				Val: param{
					kind: "p9",
					job:  "randwrite-libaio-multi.job",
				},
			},
			testing.Param{
				Name: "block_randwrite_libaio",
				Val: param{
					kind: "block",
					job:  "randwrite-libaio.job",
				},
			},
			testing.Param{
				Name: "virtiofs_randwrite_libaio",
				Val: param{
					kind: "fs",
					job:  "randwrite-libaio.job",
				},
			},
			testing.Param{
				Name: "p9_randwrite_libaio",
				Val: param{
					kind: "p9",
					job:  "randwrite-libaio.job",
				},
			},
			testing.Param{
				Name: "block_seqwrite_psync",
				Val: param{
					kind: "block",
					job:  "seqwrite-psync.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqwrite_psync",
				Val: param{
					kind: "fs",
					job:  "seqwrite-psync.job",
				},
			},
			testing.Param{
				Name: "p9_seqwrite_psync",
				Val: param{
					kind: "p9",
					job:  "seqwrite-psync.job",
				},
			},
			testing.Param{
				Name: "block_seqread_psync_multi",
				Val: param{
					kind: "block",
					job:  "seqread-psync-multi.job",
				},
			},
			testing.Param{
				Name: "virtiofs_seqread_psync_multi",
				Val: param{
					kind: "fs",
					job:  "seqread-psync-multi.job",
				},
			},
			testing.Param{
				Name: "p9_seqread_psync_multi",
				Val: param{
					kind: "p9",
					job:  "seqread-psync-multi.job",
				},
			},
		},
	})
}

func Fio(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.Blogbench.")
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
		"-m", "256",
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
