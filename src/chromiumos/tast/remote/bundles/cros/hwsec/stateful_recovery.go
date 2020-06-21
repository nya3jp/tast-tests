// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StatefulRecovery,
		Desc: "Checks that StatefulRecovery feature works",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"tpm"},
	})
}

const (
	// flagFileLoc is the location of the decrypt_stateful flag file.
	flagFileLoc = "/mnt/stateful_partition/decrypt_stateful"

	// waitForDeviceRebootTimeout is the time we wait for the device to reboot or to come back online.
	waitForDeviceRebootTimeout = 60 * time.Second

	// testFilename is the filename of the test file under user's home directory.
	testFilename = "testing.txt"

	// testContent is the test file content.
	testContent = "StatefulRecoveryTest"

	// decryptedPath is the directory that holds the decrypted data.
	decryptedPath = "/mnt/stateful_partition/decrypted/"

	// testFileSubpath is the relative path of the test file in the decrypted directory.
	testFileSubpath = "mount/user"
)

// createUserAndFlag is phase 1 of the test, it mounts the user's vault and write the test file, then it writes the decrypt_stateful file that triggers stateful recovery to the disk.
func createUserAndFlag(ctx context.Context, r hwsec.CmdRunner, utility *hwsec.UtilityCryptohomeBinary, username, pass string) error {
	// Create the user that we want to rescue
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create user vault")
	}
	defer func() {
		if err := utility.UnmountAll(ctx); err != nil {
			// It's not that serious so we won't fail the test here.
			testing.ContextLog(ctx, "Failed to unmount vault in createUserAndFlag(): ", err)
		}
	}()

	// Write test file to user's directory
	out, err := r.Run(ctx, "cryptohome-path", "user", util.FirstUsername)
	if err != nil {
		return errors.Wrap(err, "failed to get user home path")
	}
	userHome := strings.TrimSpace(string(out))
	testPath := filepath.Join(userHome, testFilename)
	if _, err := r.Run(ctx, "sh", "-c", fmt.Sprintf("echo %q > %q", testContent, testPath)); err != nil {
		return errors.Wrap(err, "failed to write test file")
	}

	// Write out a V2 decrypt_stateful file content.
	// Note that the following command is taken mostly from recovery_init.sh in order to mimic the way it is done in practice.
	cmdline := fmt.Sprintf("username=%q; password=%q; salt=$(hexdump -v -e '/1 \"%%02x\"' </mnt/stateful_partition/home/.shadow/salt); passkey=$(printf '%%s' \"${salt}${password}\" | sha256sum | cut -c-32); cat > %q <<EOM\n2\n$username\n$passkey\nEOM", username, pass, flagFileLoc)
	if _, err := r.Run(ctx, "sh", "-c", cmdline); err != nil {
		return errors.Wrap(err, "failed to create decrypt_stateful file")
	}

	return nil
}

// waitForDeviceToReboot waits for the device to go offline. Timeout is set by context.
func waitForDeviceToReboot(ctx context.Context, r hwsec.CmdRunner) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// We don't want to wait too long here.
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Running /bin/true should always succeed when the device is online.
		if _, err := r.Run(ctx, "/bin/true"); err == nil {
			// Device is available.
			return errors.New("device still available")
		}
		return nil
	}, &testing.PollOptions{}); err != nil {
		return errors.Wrap(err, "failed to wait for device to go offline")
	}
	return nil
}

// waitForDeviceToComeBack waits for the device to come back (be accessible). Timeout is set by context.
func waitForDeviceToComeBack(ctx context.Context, r hwsec.CmdRunner, d *dut.DUT) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// We don't want to wait too long here.
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		if err := d.WaitConnect(ctx); err != nil {
			return errors.Wrap(err, "failed to connect to DUT")
		}
		// Running /bin/true should always succeed when the device is online.
		if _, err := r.Run(ctx, "/bin/true"); err != nil {
			// Device is unavailable.
			return errors.Wrap(err, "device still unavailable")
		}
		return nil
	}, &testing.PollOptions{}); err != nil {
		return errors.Wrap(err, "failed to wait for device to come online")
	}
	return nil
}

// rebootAndWaitForRecovery is phase 2 of the test, it reboot and trigger the stateful recovery process, then wait for the process to finish.
func rebootAndWaitForRecovery(ctx context.Context, r hwsec.CmdRunner, utilty *hwsec.UtilityCryptohomeBinary, d *dut.DUT) (retErr error) {
	if err := d.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot")
	}

	// Wait for stateful recovery to finish. When it finished, recovery_request would be set to 1.
	const waitForStatefulRecoveryFinishTimeout = 120 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := r.Run(ctx, "crossystem", "recovery_request")
		if err != nil {
			return errors.Wrap(err, "failed to run crossystem recovery_request")
		}
		msg := strings.TrimSpace(string(out))
		if msg != "1" {
			return errors.Errorf("recovery_request incorrect, expected 1, got %q", msg)
		}
		return nil
	}, &testing.PollOptions{Timeout: waitForStatefulRecoveryFinishTimeout}); err != nil {
		return errors.Wrap(err, "failed to wait for stateful recovery finish")
	}

	// Note that there's a 20 seconds gap inserted in cryptohome's stateful recovery in order to allow for resetting recovery_request and delete the flag file.
	if err := cleanupRequestRecoveryAndDecryptStateful(ctx, r); err != nil {
		return errors.Wrap(err, "failed to cleanup after stateful recovery")
	}

	// Wait for the system to reboot/go offline and come back online.
	testing.ContextLog(ctx, "Stateful Recovery completed, now wait for device to reboot by itself")
	func() {
		ctx, cancel := context.WithTimeout(ctx, waitForDeviceRebootTimeout)
		defer cancel()
		if err := waitForDeviceToReboot(ctx, r); err != nil {
			retErr = errors.Wrap(err, "failed to wait for device to go offline (it probably didn't reboot)")
		}
	}()
	if retErr != nil {
		return retErr
	}
	testing.ContextLog(ctx, "DUT rebooted, now wait for it to come back")
	func() {
		ctx, cancel := context.WithTimeout(ctx, waitForDeviceRebootTimeout)
		defer cancel()
		if err := waitForDeviceToComeBack(ctx, r, d); err != nil {
			retErr = errors.Wrap(err, "failed to wait for device to come back")
		}
	}()
	if retErr != nil {
		return retErr
	}
	testing.ContextLog(ctx, "DUT is back")

	return nil
}

// cleanupRequestRecoveryAndDecryptStateful is a cleanup function that removes the decrypt_stateful flag file and the recovery_request flag.
func cleanupRequestRecoveryAndDecryptStateful(ctx context.Context, r hwsec.CmdRunner) (retErr error) {
	// Note that we go through both actions even if one failed.

	// Remove the flag file.
	if _, err := r.Run(ctx, "rm", "-f", flagFileLoc); err != nil {
		testing.ContextLog(ctx, "Unable to remove the flag file: ", err)
		retErr = errors.Wrap(err, "failed to remove the flag file")
	}
	// Set recovery_request back to 0 because we don't want recovery to be triggered.
	if _, err := r.Run(ctx, "crossystem", "recovery_request=0"); err != nil {
		testing.ContextLog(ctx, "Failed to reset recovery_request: ", err)
		retErr = errors.Wrap(err, "failed to reset recovery_request, the system may reboot into recovery on the next boot")
	}
	return retErr
}

// verifyTestFile is phase 3 of the test, it verifies that the contents of the user's home directory have indeed been recovered.
func verifyTestFile(ctx context.Context, r hwsec.CmdRunner) (retErr error) {
	out, err := r.Run(ctx, "cat", filepath.Join(decryptedPath, testFileSubpath, testFilename))
	if err != nil {
		return errors.Wrap(err, "failed to read the test file")
	}

	msg := strings.TrimSpace(string(out))
	if msg != testContent {
		return errors.Errorf("test file content mismatch, expected %q, got %q", testContent, msg)
	}

	return nil
}

// StatefulRecovery tests the stateful recovery functionality.
func StatefulRecovery(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	// Stage 1: Create user vault and test file, then write the stateful recovery request file.
	if err := createUserAndFlag(ctx, r, utility, util.FirstUsername, util.FirstPassword); err != nil {
		s.Fatal("Stage 1 failed: ", err)
	}
	defer func() {
		// It is possible for decrypt_stateful flag or recovery_request to remain on the system after the test should things go wrong.
		// Those remaining pieces could cause problems for tests that happens later, so we need to get rid of them.

		// Wait for device to be available.
		if err := waitForDeviceToComeBack(ctx, r, s.DUT()); err != nil {
			s.Log("Failed to wait for device to come back online during cleanup: ", err)
		}

		// Do the cleanup.
		if err := cleanupRequestRecoveryAndDecryptStateful(ctx, r); err != nil {
			s.Log("Failed to cleanup at the end of test: ", err)
		}

		// Unmount and remove the vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Log("Failed to unmount during cleanup: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Log("Failed to remove vault during cleanup: ", err)
		}

		// Remove the decrypted folder
		if _, err := r.Run(ctx, "rm", "-rf", decryptedPath); err != nil {
			s.Log("Failed to remove the decrypted output directory: ", err)
		}
	}()

	// Stage 2: Reboot and wait for stateful to finish.
	if err := rebootAndWaitForRecovery(ctx, r, utility, s.DUT()); err != nil {
		s.Fatal("Stage 2 failed: ", err)
	}

	// Stage 3: Verify the content of the test file.
	if err := verifyTestFile(ctx, r); err != nil {
		s.Fatal("Stage 3 failed: ", err)
	}
}
