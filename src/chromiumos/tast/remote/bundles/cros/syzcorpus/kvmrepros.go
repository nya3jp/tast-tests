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

	cnt := 1
	for _, f := range files {
		fname := f.Name()
		if _, ok := enabledRepros[fname]; !ok {
			s.Log("Skipping ", fname)
			continue
		}

		s.Logf("=> Using repro(%v/%v): %v", cnt, len(enabledRepros), fname)
		localPath := filepath.Join(binDir, fname)
		remotePath := filepath.Join("/usr/local/tmp", fname)

		if err := syzutils.CopyRepro(ctx, d, localPath, remotePath); err != nil {
			s.Fatal("Failed to copy repro: ", err)
		}

		if out, err := syzutils.RunRepro(ctx, d, remotePath, 5*time.Second); err != nil {
			s.Logf("RunRepro returned %v: with combined output: %v", err, out)
		}

		if err := syzutils.KillRepro(ctx, d, fname); err != nil {
			s.Log("KillRepro failed: ", err)
		}

		didWarn, err := syzutils.WarningInDmesg(ctx, d)
		if err != nil {
			s.Fatal("warningInDmesg failed: ", err)
		} else if didWarn {
			// TODO: Copy the warning log.
			s.Fatalf("Warning found at sample %v, resetting DUT", fname)
			if err := d.Reboot(ctx); err != nil {
				s.Fatal("Failed to reboot DUT: ", err)
			}
		}
		if err := syzutils.ClearDmesg(ctx, d); err != nil {
			s.Fatal("Unable to clear dmesg: ", err)
		}

		cnt++
	}

	s.Log("Finished running all repros in ", time.Since(start))
	// TODO: copy pstore logs if the DUT reboots in between testing.

	s.Log("Done testing, exiting")
}
