// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	//"chromiumos/tast/ctxutil"
	//"chromiumos/tast/dut"
	//"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	//"chromiumos/tast/rpc"
	//"chromiumos/tast/services/cros/arc"
	//arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SignerValidation,
		Desc: "Validates that caches match for both modes when pre-generated packages cache is enabled and disabled",
		Contacts: []string{
			"khmel@google.com",
			"arc-performance@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val: []string{
				"/opt/google/containers/android",
				"android_file_contexts",
			},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: []string{
				"/opt/google/vms/android",
				"android_file_contexts_vm",
			},
		}},
		Timeout: 15 * time.Minute,
	})
}

func SignerValidation(ctx context.Context, s *testing.State) {
	const (
		// System image file name
		systemImage = "system.raw.img"

		// Vendor image file name
		vendorImage = "vendor.raw.img"

		// Root for ARC selinux context files
		selinuxRoot = "/etc/selinux/arc/contexts/files"

		// Path to signer bin folder
		signerBinariesPath = "/mnt/host/source/src/platform/signing/signer-dev/signer/signingtools-bin"

		// Test keys root
		testKeysRoot = "/mnt/host/source/src/platform/vboot_reference/tests/devkeys/android/"
	)

	d := s.DUT()

	remoteRoot := s.Param().([]string)[0]
	selinuxContext := s.Param().([]string)[1]

	tempDir, err := ioutil.TempDir("", "tmp_dir")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := testexec.CommandContext(
		ctx,
		"sudo",
		"find",
		"/",
		"-name",
		"zipalign")
	

	// open the out file for writing
	layout, err := os.Create(filepath.Join(s.OutDir(), "layout.txt"))
	if err != nil {
		s.Fatal("Failed to create layout: ", err)
	}
	defer layout.Close()

	cmd.Stdout = layout
	cmd.Stderr = layout
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to run layout: ", err)
	}


	// Make sure required binaries exist. Signer does not treat missing binaries as an error.
	// It is handled as warning and signing is skipped if binaries are missing.
	binaries := []string{"zipalign", "apksigner", "signapk"}
	for _, binary := range binaries {
		binaryPath := filepath.Join(signerBinariesPath, binary)
		if info, err := os.Stat(binaryPath); err != nil || !info.Mode().IsRegular() {
			s.Fatalf("Failed to verify required binary exist: %q", binaryPath)
		}
	}

	resources := []string{
		filepath.Join(remoteRoot, systemImage),
		filepath.Join(remoteRoot, vendorImage),
		filepath.Join(selinuxRoot, selinuxContext),
	}

	for _, resource := range resources {
		localResource := filepath.Join(tempDir, resource)
		testing.ContextLogf(ctx, "Copying %q -> %q", resource, localResource)
		os.MkdirAll(filepath.Dir(localResource), os.ModePerm)
		if err := d.GetFile(ctx, resource, localResource); err != nil {
			s.Fatalf("Failed to get resource: %q", resource)
		}
	}

	localSystemImage := filepath.Join(tempDir, remoteRoot, systemImage)
	infoBefore, err := os.Stat(localSystemImage)
	if err != nil {
		s.Fatal("Failed to stat system image: ", err)
	}

	testing.ContextLog(ctx, "Signing image")
	cmd = testexec.CommandContext(
		ctx,
		"sudo",
		fmt.Sprintf("PATH=%s:%s", os.Getenv("PATH"), signerBinariesPath),
		"/mnt/host/source/src/platform/vboot_reference/scripts/image_signing/sign_android_image.sh",
		tempDir,
		testKeysRoot)
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s:%s", os.Getenv("PATH"), signerBinariesPath))
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start signer: ", err)
	}

	if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run signer: ", err)
	}

	// Sanity check that image was actually modified. In very few cases, like missing required
	// binaries, signer script generates warn and exits successfully.
	infoAfter, err := os.Stat(localSystemImage)
	if err != nil {
		s.Fatal("Failed to stat system image: ", err)
	}
	if infoAfter.ModTime() == infoBefore.ModTime() {
		s.Fatal("Signing is done but system image was not updated")
	}
}
