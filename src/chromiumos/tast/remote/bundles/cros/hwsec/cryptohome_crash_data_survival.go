// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/storage/files"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

const (
	// waitForCryptohomedTimeout is the timeout waiting for cryptohomed to respawn.
	waitForCryptohomedTimeout = 30 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeCrashDataSurvival,
		Desc: "Checks when cryptohome crashed or is forcefully killed, user's data are not lost",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

// getCryptohomedPID query the cryptohomed's PID.
func getCryptohomedPID(ctx context.Context, r hwsec.CmdRunner) (int, error) {
	raw, err := r.Run(ctx, "pidof", "cryptohomed")
	if err != nil {
		return -1, errors.Wrap(err, "failed to run pidof to get cryptohomed pid")
	}
	out := strings.TrimSpace(string(raw))
	pid, err := strconv.Atoi(out)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to parse pid from str %q", out)
	}
	return pid, nil
}

func CryptohomeCrashDataSurvival(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()
	dc := hwsec.NewDaemonController(r)

	// Clear any remnant data on the DUT.
	utility.UnmountAndRemoveVault(ctx, util.FirstUsername)

	// Create a user vault for testing.
	if err := utility.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user: ", err)
	}
	defer func() {
		if err := utility.UnmountAndRemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to remove user vault: ", err)
		}
	}()

	// Create test files
	hf, err := files.NewHomedirFiles(ctx, utility, r, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for testing files in user's home directory: ", err)
	}
	if err = hf.Clear(ctx); err != nil {
		s.Fatal("Failed to clear test files in the user's home directory: ", err)
	}
	if err = hf.StepAll(ctx); err != nil {
		s.Fatal("Failed to initialize the test files in the user's home directory: ", err)
	}

	// Get cryptohomed's current pid.
	lastPid, err := getCryptohomedPID(ctx, r)
	if err != nil {
		s.Fatal("Failed to get cryptohomed's pid before kill: ", err)
	}

	// Kill cryptohomed and wait for it to restart
	if _, err = r.Run(ctx, "killall", "-SIGKILL", "cryptohomed"); err != nil {
		s.Fatal("Failed to kill cryptohomed")
	}

	// We need to wait for the new pid to appear, because it takes dbus some time to realize that
	// the cryptohome dbus service is gone.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		pid, err := getCryptohomedPID(ctx, r)
		if err != nil {
			return errors.Wrap(err, "cryptohomed pid unavailable")
		}
		if pid == lastPid {
			return errors.Errorf("cryptohomed pid %d did not change", pid)
		}
		return nil
	}, &testing.PollOptions{Timeout: waitForCryptohomedTimeout}); err != nil {
		s.Fatal("Failed to wait for cryptohomed to come back: ", err)
	}

	// Wait for Cryptohomed service to come back.
	if err := dc.WaitForAllDBusServices(ctx); err != nil {
		s.Fatal("DBus services did not return: ", err)
	}
	// Cryptohome is back

	// Unmount and check that the files are no longer accessible.
	if err = utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount all: ", err)
	}
	if err = hf.VerifyAll(ctx); err == nil {
		s.Error("Files still visible after Unmount() post crash")
	}

	// Mount again and things should be fine.
	if err = utility.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user post crash: ", err)
	}
	if err = hf.VerifyAll(ctx); err != nil {
		s.Fatal("Files invalid after remount post crash: ", err)
	}
	if err = hf.StepAll(ctx); err != nil {
		s.Fatal("Unable to write files after remount post crash: ", err)
	}

	// Unmount before restart.
	// Note that if we unmount here, we'll not test the case of restarting cryptohome when a vault is mounted.
	// TODO(b/205502383): Add testing of unclean cryptohome shutdown and subsequent mount.
	if err = utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount all: ", err)
	}

	// Restart all daemons to simulate a reboot.
	if err := dc.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM daemons: ", err)
	}
	if err = utility.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user post restart: ", err)
	}
	if err = hf.VerifyAll(ctx); err != nil {
		s.Fatal("Files invalid after mount post restart: ", err)
	}
	if err = hf.StepAll(ctx); err != nil {
		s.Fatal("Unable to write files after mount post restart: ", err)
	}
}
