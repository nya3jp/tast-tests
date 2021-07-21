// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EncryptedRebootVault,
		Desc: "Checks that the encrypted reboot vault is setup correctly and survives a reboot",
		Contacts: []string{
			"sarthakkukreti@google.com",
			"chromeos-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pstore", "reboot"},
	})
}

func validateRebootVault(ctx context.Context, d *dut.DUT) error {
	return d.Conn().CommandContext(ctx, "encrypted-reboot-vault", "--action=validate").Run()
}

func EncryptedRebootVault(ctx context.Context, s *testing.State) {
	d := s.DUT()

	const (
		encryptedVaultFileContents = "encrypted_vault_contents"
		encryptedVaultFilePath     = "/mnt/stateful_partition/reboot_vault/test_file"
	)

	// Check that the encrypted reboot vault is setup correctly.
	if err := validateRebootVault(ctx, d); err != nil {
		s.Fatal("Unable to validate the encrypted reboot vault: ", err)
	}

	// Create a file in the encrypted reboot vault.
	// TODO(crbug.com/1047737): Replace with linuxssh.WriteFile() once available.
	cmd := fmt.Sprintf("echo '%s' >  %s", encryptedVaultFileContents, encryptedVaultFilePath)
	if out, err := d.Conn().CommandContext(ctx, "sh", "-c", cmd).CombinedOutput(); err != nil {
		s.Fatalf("Unable to create test file: %s", out)
	}

	defer func() {
		if err := d.Conn().CommandContext(ctx, "rm", "-f", encryptedVaultFilePath).Run(); err != nil {
			s.Fatal("Failed to clean up test file: ", err)
		}
	}()

	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	// Check again that the encrypted reboot vault was set up correctly.
	if err := validateRebootVault(ctx, d); err != nil {
		s.Fatal("Unable to validate the encrypted reboot vault: ", err)
	}

	// Check the contents of the test file added.
	out, err := d.Conn().CommandContext(ctx, "cat", encryptedVaultFilePath).Output()
	if err != nil {
		s.Fatal("Failed to read file: ", err)
	}

	if !strings.Contains(string(out), encryptedVaultFileContents) {
		s.Fatalf("File contents do not match: %s %s", string(out), encryptedVaultFileContents)
	}
}
