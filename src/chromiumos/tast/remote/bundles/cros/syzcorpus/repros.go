// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syzcorpus

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/syzcorpus/syzutils"
	"chromiumos/tast/testing"
)

const (
	binKVMX64Zip     = "bin_kvm_x86_64.zip"
	kvmEnabledRepros = "kvm_x86_64.txt"

	binBlockX64Zip     = "bin_block_x86_64.zip"
	blockEnabledRepros = "block_x86_64.txt"

	binJbd2X64Zip     = "bin_jbd2_x86_64.zip"
	jbd2EnabledRepros = "jbd2_x86_64.txt"

	binFuseX64Zip     = "bin_fuse_x86_64.zip"
	fuseEnabledRepros = "fuse_x86_64.txt"

	binBpfX64Zip     = "bin_bpf_x86_64.zip"
	bpfEnabledRepros = "bpf_x86_64.txt"
)

type testParam struct {
	subsystem   string
	binariesZip string
	reprosList  string
	windowSize  int
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Repros,
		Desc: "Test that runs syzkaller repros",
		Contacts: []string{
			"zsm@chromium.org", // Test author
			"chromeos-kernel@google.com",
		},
		Timeout: 30 * time.Minute,
		Attr:    []string{"group:syzcorpus"},
		Params: []testing.Param{
			{
				Name: "kvm",
				Val: testParam{
					subsystem:   "kvm",
					binariesZip: binKVMX64Zip,
					reprosList:  kvmEnabledRepros,
					windowSize:  5,
				},
				ExtraData: []string{binKVMX64Zip, kvmEnabledRepros},
			},
			{
				Name: "block",
				Val: testParam{
					subsystem:   "block",
					binariesZip: binBlockX64Zip,
					reprosList:  blockEnabledRepros,
					windowSize:  5,
				},
				ExtraData: []string{binBlockX64Zip, blockEnabledRepros},
			},
			{
				Name: "jbd2",
				Val: testParam{
					subsystem:   "jbd2",
					binariesZip: binJbd2X64Zip,
					reprosList:  jbd2EnabledRepros,
					windowSize:  5,
				},
				ExtraData: []string{binJbd2X64Zip, jbd2EnabledRepros},
			},
			{
				Name: "fuse",
				Val: testParam{
					subsystem:   "fuse",
					binariesZip: binFuseX64Zip,
					reprosList:  fuseEnabledRepros,
					windowSize:  5,
				},
				ExtraData: []string{binFuseX64Zip, fuseEnabledRepros},
			},
			{
				Name: "bpf",
				Val: testParam{
					subsystem:   "bpf",
					binariesZip: binBpfX64Zip,
					reprosList:  bpfEnabledRepros,
					windowSize:  1,
				},
				ExtraData: []string{binBpfX64Zip, bpfEnabledRepros},
			},
		},
	})
}

// Repros runs syzkaller repros against the DUT.
func Repros(ctx context.Context, s *testing.State) {
	start := time.Now()
	d := s.DUT()

	param := s.Param().(testParam)
	s.Log("Running repros from: ", param.reprosList)

	arch, err := syzutils.FindDUTArch(ctx, d)
	if err != nil {
		s.Fatal("Unable to find syzkaller arch: ", err)
	}
	s.Log("Arch found to be: ", arch)

	tastDir, err := ioutil.TempDir("", "tast-syzcorpus")
	if err != nil {
		s.Fatal("Unable to create tast temporary directory: ", err)
	}
	defer os.RemoveAll(tastDir)

	crashesDir := filepath.Join(s.OutDir(), "crashes")
	if err := os.Mkdir(crashesDir, 0755); err != nil {
		s.Fatal("Unable to create temp crashes dir: ", err)
	}

	// Read enabled repros.
	enabledRepros, err := syzutils.LoadEnabledRepros(s.DataPath(param.reprosList))
	if err != nil {
		s.Fatal("Unable to load enabled repros: ", err)
	}

	// Extract corpus.
	s.Log("Extracting syzkaller corpus")
	if err := syzutils.ExtractCorpus(ctx, tastDir, s.DataPath(param.binariesZip)); err != nil {
		s.Fatal("Encountered error fetching fuzz artifacts: ", err)
	}
	binDir := filepath.Join(tastDir, fmt.Sprintf("bin_%v_%v", param.subsystem, arch))
	files, err := ioutil.ReadDir(binDir)
	if err != nil {
		s.Fatalf("Unable to read extracted corpus dir at: %v: %v", binDir, err)
	}

	// Clear dmesg before starting to test.
	if err := syzutils.ClearDmesg(ctx, d); err != nil {
		s.Fatal("Unable to clear dmesg: ", err)
	}

	var repros []string
	for _, f := range files {
		fname := f.Name()
		if _, ok := enabledRepros[fname]; !ok {
			s.Log("Skipping ", fname)
			continue
		}
		repros = append(repros, fname)
	}

	count := 1
	windowSize := param.windowSize
	for start := 0; start < len(repros); start += windowSize {
		// Take windowSize number of repros at a time.
		end := start + windowSize
		if end > len(repros) {
			end = len(repros)
		}
		// Execute windowSize number of repros in parallel.
		group, c := errgroup.WithContext(ctx)
		for _, repro := range repros[start:end] {
			r := repro
			s.Logf("=> Using repro(%v/%v): %v", count, len(repros), r)
			group.Go(func() error {
				return worker(c, d, binDir, r)
			})
			count++
		}
		// Wait for windowSize repros to finish, and check if any errors were
		// encountered.
		if err := group.Wait(); err != nil {
			s.Fatal("Received error from worker: ", err)
		}
		// Check dmesg for any warnings or errors.
		warning, err := syzutils.WarningInDmesg(ctx, d)
		if err != nil {
			s.Fatalf("warningInDmesg failed after running repros %v: %v", repros[start:end], err)
		} else if len(warning) > 0 {
			crashFile := filepath.Join(crashesDir, "stacktrace")
			if err := ioutil.WriteFile(crashFile, warning, 0755); err != nil {
				s.Log("Failed to save warning: ", err)
			}
			if err := d.Reboot(ctx); err != nil {
				s.Fatal("Failed to reboot DUT: ", err)
			}
			s.Fatalf("Warning found with repros %v, DUT reset", repros[start:end])
		}
		if err := syzutils.ClearDmesg(ctx, d); err != nil {
			s.Fatal("Unable to clear dmesg: ", err)
		}
	}
	s.Log("Finished running all repros in ", time.Since(start))
}

func worker(ctx context.Context, d *dut.DUT, binDir, repro string) error {
	localPath := filepath.Join(binDir, repro)
	remoteDir := filepath.Join("/usr/local/tmp", repro)
	if err := syzutils.MkdirRemote(ctx, d, remoteDir); err != nil {
		return errors.Wrapf(err, "unable to create temp repro dir for %v", repro)
	}
	defer syzutils.RmdirRemote(ctx, d, remoteDir)
	remotePath := filepath.Join(remoteDir, repro)
	if err := syzutils.CopyRepro(ctx, d, localPath, remotePath); err != nil {
		return errors.Wrapf(err, "failed to copy repro %v", repro)
	}
	if out, err := syzutils.RunRepro(ctx, d, remotePath, 5*time.Second); err != nil {
		testing.ContextLogf(ctx, "RunRepro returned %v: with combined output: %v", err, string(out))
	}
	return nil
}
