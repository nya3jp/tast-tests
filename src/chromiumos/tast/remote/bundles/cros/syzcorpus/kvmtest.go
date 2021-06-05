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
	binKvmAmd64Zip   = "bin_kvm_amd64.zip"
	kvmEnabledRepros = "kvm_amd64.txt"
	dutSSHKey        = "testing_rsa"
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
		Data:    []string{dutSSHKey, binKvmAmd64Zip, kvmEnabledRepros},
	})
}

// Kvmtest runs Syzkaller repros against the DUT.
func Kvmtest(ctx context.Context, s *testing.State) {
	start := time.Now()

	tc := syzutils.NewTestContext(ctx, s.DUT(), dutSSHKey)

	syzArch, err := tc.FindSyzkallerArch()
	if err != nil {
		s.Fatal("Unable to find syzkaller arch: ", err)
	}
	s.Log("syzArch found to be: ", syzArch)

	tastDir, err := ioutil.TempDir("", "tast-syzcorpus")
	if err != nil {
		s.Fatal("Unable to create tast temporary directory: ", err)
	}
	defer os.RemoveAll(tastDir)

	// Extract corpus.
	s.Log("Extracting syzkaller corpus")
	if err := syzutils.ExtractCorpus(tastDir, s.DataPath(binKvmAmd64Zip)); err != nil {
		s.Fatal("Encountered error fetching fuzz artifacts: ", err)
	}

	// Chmod the keyfile so that ssh connections do not fail due to
	// open permissions.
	sshKey := s.DataPath(dutSSHKey)
	if err := os.Chmod(sshKey, 0600); err != nil {
		s.Fatal("Unable to chmod sshkey to 0600: ", err)
	}

	// Read enabled repros.
	enabledRepros, err := syzutils.LoadEnabledRepros(s.DataPath(kvmEnabledRepros))
	if err != nil {
		s.Fatal("Unable to load disabled repros: ", err)
	}

	binDir := filepath.Join(tastDir, fmt.Sprintf("bin_kvm_%v", syzArch))
	files, err := ioutil.ReadDir(binDir)
	if err != nil {
		s.Fatalf("Unable to read extracted corpus dir at: %v: %v", binDir, err)
	}

	if err := tc.RemountTmp(s); err != nil {
		s.Fatal("remountTmp failed: ", err)
	}

	for idx, f := range files {
		fname := f.Name()
		if _, ok := enabledRepros[fname]; !ok {
			s.Log("Skipping ", fname)
			continue
		}

		s.Logf("=> Using repro(%v/%v): %v", idx, len(enabledRepros), fname)
		localPath := filepath.Join(binDir, fname)
		remotePath := filepath.Join("/tmp", fname)

		if err := tc.CopyRepro(s, localPath, remotePath); err != nil {
			s.Fatal("Failed to copy repro: ", err)
		}
		if err := tc.RunRepro(s, remotePath, 5*time.Second); err != nil {
			s.Fatal("Failed to run repro: ", err)
		}

		// Running the repro might cause the DUT to reboot unexpectedly at any
		// point.
		didWarn, err := tc.WarningInDmesg(s)
		if err != nil {
			s.Fatal("warningInDmesg failed: ", err)
		} else if didWarn {
			s.Fatalf("Warning found at sample %v, resetting DUT", fname)
			tc.ResetDUT(s, true)
		}
		if err := tc.ClearDmesg(s); err != nil {
			s.Fatal("Unable to clear dmesg: ", err)
		}
	}

	s.Log("Finished running all repros in ", time.Since(start))
	// TODO: copy pstore logs if the DUT reboots in between testing.
	tc.ResetDUT(s, false)
	s.Log("Done testing, exiting")
}
