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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/syzcorpus/syzutils"
	"chromiumos/tast/testing"
)

const (
	binKVMX64Zip     = "bin_kvm_x86_64.zip"
	kvmEnabledRepros = "kvm_x86_64.txt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KVMRepros,
		Desc: "Test that runs KVM repros",
		Contacts: []string{
			"zsm@chromium.org", // Test author
			"chromeos-kernel@google.com",
		},
		Timeout: 30 * time.Minute,
		Attr:    []string{"group:syzcorpus"},
		Data:    []string{binKVMX64Zip, kvmEnabledRepros},
	})
}

// KVMRepros runs KVM syzkaller repros against the DUT.
func KVMRepros(ctx context.Context, s *testing.State) {
	start := time.Now()
	d := s.DUT()

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

	crashesDir := filepath.Join(tastDir, "crashes")
	if err := os.Mkdir(crashesDir, 0755); err != nil {
		s.Fatal("Unable to create temp crashes dir: ", err)
	}
	defer func() {
		if err := testexec.CommandContext(ctx, "cp", "-r", crashesDir, s.OutDir()).Run(); err != nil {
			s.Log("Unable to save crashDir: ", err)
		}
	}()

	// Read enabled repros.
	enabledRepros, err := syzutils.LoadEnabledRepros(s.DataPath(kvmEnabledRepros))
	if err != nil {
		s.Fatal("Unable to load enabled repros: ", err)
	}

	// Extract corpus.
	s.Log("Extracting syzkaller corpus")
	if err := syzutils.ExtractCorpus(ctx, tastDir, s.DataPath(binKVMX64Zip)); err != nil {
		s.Fatal("Encountered error fetching fuzz artifacts: ", err)
	}
	binDir := filepath.Join(tastDir, fmt.Sprintf("bin_kvm_%v", arch))
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
	windowSize := 5
	for start := 0; start < len(repros); start += windowSize {
		// Take windowSize number of repros at a time.
		end := start + windowSize
		if end > len(repros) {
			end = len(repros)
		}
		errChan := make(chan error, end-start)
		// Execute windowSize number of repros in parallel.
		for _, repro := range repros[start:end] {
			s.Logf("=> Using repro(%v/%v): %v", count, len(repros), repro)
			go worker(ctx, d, binDir, repro, errChan)
			count++
		}
		// Wait for windowSize repros to finish, and check if any errors were
		// encountered.
		for i := 0; i < end-start; i++ {
			err := <-errChan
			if err != nil {
				s.Fatal("Received error from worker: ", err)
			}
		}
		// Check dmesg for any warnings or errors.
		warning, err := syzutils.WarningInDmesg(ctx, d)
		if err != nil {
			s.Fatalf("warningInDmesg failed after running repros %v: %v", repros[start:end], err)
		} else if warning != nil {
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

func worker(ctx context.Context, d *dut.DUT, binDir, repro string, errChan chan error) {
	localPath := filepath.Join(binDir, repro)
	remotePath := filepath.Join("/usr/local/tmp", repro)
	if err := syzutils.CopyRepro(ctx, d, localPath, remotePath); err != nil {
		testing.ContextLog(ctx, "Failed to copy repro: ", err)
		errChan <- errors.Wrapf(err, "failed to copy repro %v", repro)
		return
	}
	if out, err := syzutils.RunRepro(ctx, d, remotePath, 5*time.Second); err != nil {
		testing.ContextLogf(ctx, "RunRepro returned %v: with combined output: %v", err, string(out))
	}
	errChan <- nil
}
