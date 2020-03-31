// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

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
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"reboot"},
	})
}

func validateRebootVault(ctx context.Context, d *dut.DUT) error {
	return d.Command("sh", "-c", "encrypted-reboot-vault --action=validate").Run(ctx)
}

func EncryptedRebootVault(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Check that the encrypted reboot vault is setup correctly.
	if err := validateRebootVault(ctx, d); err != nil {
		s.Fatal("Unable to validate the encrypted reboot vault: ", err)
	}

	// Create a file in the encrypted reboot vault.
	if err := d.Command("sh", "-c", "echo 'hello' > /mnt/stateful_partition/reboot_vault/test_file").Run(ctx); err != nil {
		s.Fatal("Unable to create test file: ", err)
	}

	defer func() {
		if err := d.Command("rm", "-rf", "/mnt/stateful_partition/reboot_vault/test_file").Run(ctx); err != nil {
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
	if err := d.Command("sh", "-c", "grep -q 'hello' /mnt/stateful_partition/reboot_vault/test_file").Run(ctx); err != nil {
		s.Fatal("File contents do not match: ", err)
	}
}
