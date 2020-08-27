// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

const (
	cryptohomeDir            = "/run/cryptohome"
	cryptohomeStartingFile   = "cryptohome-starting"
	cryptohomeRegisteredFile = "cryptohome-registered"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomePerf,
		Desc: "Cryptohome performance test",
		Attr: []string{
			"crosbolt_perbuild",
			"group:hwsec_destructive_crosbolt",
		},
		Contacts: []string{
			"yich@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"reboot", "tpm"},
	})
}

// waitUntilCryptohomeInit is a helper function to wait until cryptohome initialized.
func waitUntilCryptohomeInit(ctx context.Context, remote *hwsecremote.CmdRunnerRemote) error {
	return testing.Poll(ctx, func(context.Context) error {
		// Check that bootstat files are available.
		for _, path := range []string{cryptohomeStartingFile, cryptohomeRegisteredFile} {
			file := filepath.Join(cryptohomeDir, path)
			_, err := remote.Run(ctx, "stat", file)
			if err != nil {
				return errors.Wrap(err, "stat file failed")
			}
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: time.Second,
	})
}

// getTimestamp returns timestamp for a cryptohome event.
func getTimestamp(ctx context.Context, remote *hwsecremote.CmdRunnerRemote, eventName string) (float64, error) {
	eventFile := filepath.Join(cryptohomeDir, eventName)
	data, err := remote.Run(ctx, "stat", "--format=%.Y", eventFile)
	if err != nil {
		return 0.0, err
	}
	trimmedData := strings.TrimSpace(string(data))

	stamp, err := strconv.ParseFloat(trimmedData, 64)
	if err != nil {
		return 0.0, err
	}
	return stamp, nil
}

func clearOwnership(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	helper, err := hwsecremote.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	if result, err := utility.IsPreparedForEnrollment(ctx); err != nil {
		s.Fatal("Cannot check if enrollment preparation is reset: ", err)
	} else if result {
		s.Fatal("Enrollment preparation is not reset after clearing ownership")
	}
	s.Log("Enrolling with TPM not ready")
	if _, err := utility.CreateEnrollRequest(ctx, hwsec.DefaultPCA); err == nil {
		s.Fatal("Enrollment should not happen w/o getting prepared")
	}
}

func CryptohomePerf(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("Failed to create new command runner: ", err)
	}

	clearOwnership(ctx, s)

	err = waitUntilCryptohomeInit(ctx, r)
	if err != nil {
		s.Fatal("Failed to wait boot complete: ", err)
	}

	cryptohomeStarting, err := getTimestamp(ctx, r, cryptohomeStartingFile)
	if err != nil {
		s.Fatal("Failed to parse cryptohome uptime: ", err)
	}

	cryptohomeRegistered, err := getTimestamp(ctx, r, cryptohomeRegisteredFile)
	if err != nil {
		s.Fatal("Failed to parse cryptohome-internal uptime: ", err)
	}

	// Record the perf measurements.
	value := perf.NewValues()

	// Because cryptohome depend on tpm_manager and chaps,
	// the start-up time of cryptohome is the difference between the
	// cryptohome started timestamp and the latest timestamp of two daemons.
	startUpTime := cryptohomeRegistered - cryptohomeStarting

	s.Log("start-up time of cryptohome: ", startUpTime)

	value.Set(perf.Metric{
		Name:      "crpytohome_start_time",
		Variant:   "oobe_dbus",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, startUpTime)

	value.Save(s.OutDir())
}
