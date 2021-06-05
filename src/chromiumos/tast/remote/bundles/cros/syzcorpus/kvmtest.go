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

	"chromiumos/tast/testing"
)

const (
	BIN_KVM_AMD64_ZIP  = "bin_kvm_amd64.zip"
	KVM_ENABLED_REPROS = "kvm_amd64.txt"
	DUT_SSH_KEY        = "testing_rsa"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Kvmtest,
		Desc: "Wrapper that runs KVM repros",
		Contacts: []string{
			"zsm@chromium.org", // Test author
			"chromeos-kernel@google.com",
		},
		Timeout: 500 * time.Minute,
		Data:    []string{DUT_SSH_KEY, BIN_KVM_AMD64_ZIP, KVM_ENABLED_REPROS},
	})
}

// Kvmtest runs Syzkaller repros against the DUT.
func Kvmtest(ctx context.Context, s *testing.State) {
	start := time.Now()

	tc := NewTestContext(ctx, s)

	syzArch, err := tc.findSyzkallerArch()
	if err != nil {
		s.Fatalf("Unable to find syzkaller arch: %v", err)
	}
	s.Logf("syzArch found to be: %v", syzArch)

	tastDir, err := ioutil.TempDir("", "tast-syzcorpus")
	if err != nil {
		s.Fatalf("Unable to create tast temporary directory: %v", err)
	}
	defer os.RemoveAll(tastDir)

	// Extract corpus.
	s.Log("Extracting syzkaller corpus")
	if err := extractCorpus(tastDir, s.DataPath(BIN_KVM_AMD64_ZIP)); err != nil {
		s.Fatalf("Encountered error fetching fuzz artifacts: %v", err)
	}

	// Chmod the keyfile so that ssh connections do not fail due to
	// open permissions.
	sshKey := s.DataPath(DUT_SSH_KEY)
	if err := os.Chmod(sshKey, 0600); err != nil {
		s.Fatalf("Unable to chmod sshkey to 0600: %v", err)
	}

	// Read enabled repros.
	enabledRepros, err := loadEnabledRepros(s.DataPath(KVM_ENABLED_REPROS))
	if err != nil {
		s.Fatalf("Unable to load disabled repros: %v", err)
	}

	binDir := filepath.Join(tastDir, fmt.Sprintf("bin_kvm_%v", syzArch))
	files, err := ioutil.ReadDir(binDir)
	if err != nil {
		s.Fatalf("Unable to read extracted corpus dir at: %v: %v", binDir, err)
	}

	if err := tc.remountTmp(); err != nil {
		s.Fatal(err)
	}

	for idx, f := range files {
		fname := f.Name()
		if _, ok := enabledRepros[fname]; !ok {
			s.Logf("Skipping %v", fname)
			continue
		}

		s.Logf("=> Using repro(%v/%v): %v", idx, len(enabledRepros), fname)
		localPath := filepath.Join(binDir, fname)
		remotePath := filepath.Join("/tmp", fname)

		if err := tc.copyRepro(localPath, remotePath); err != nil {
			s.Fatalf("Failed to copy repro: %v", err)
		}
		if err := tc.runRepro(remotePath, 5*time.Second); err != nil {
			s.Fatalf("Failed to run repro: %v", err)
		}

		// Running the repro might cause the DUT to reboot unexpectedly at any
		// point.
		didWarn, err := tc.warningInDmesg()
		if err != nil {
			s.Fatal(err)
		} else if didWarn {
			s.Fatalf("Warning found at sample %v, resetting DUT", fname)
			tc.resetDUT(true)
		}
		if err := tc.clearDmesg(); err != nil {
			s.Fatalf("Unable to clear dmesg: %v", err)
		}
	}

	s.Logf("Finished running all repros in %v", time.Since(start))
	// TODO: copy pstore logs if the DUT reboots in between testing.
	// Copy the syzkaller stdout/stderr logfile and the working directory
	// as part of the tast results directory.
	tc.resetDUT(false)
	s.Log("Done testing, exiting.")
}
