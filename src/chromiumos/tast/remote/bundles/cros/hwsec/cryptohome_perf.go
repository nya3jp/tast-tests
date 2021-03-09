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
	resultDir                = "/tmp"
	cryptohomeDbusTimeFile   = "cryptohome_dbus_time"
	userdataauthDbusTimeFile = "userdataauth_dbus_time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomePerf,
		Desc: "Cryptohome performance test",
		Attr: []string{
			"hwsec_destructive_crosbolt_perbuild",
			"group:hwsec_destructive_crosbolt",
		},
		Contacts: []string{
			"yich@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"reboot", "tpm"},
		ServiceDeps:  []string{"tast.cros.hwsec.AttestationDBusService"},
	})
}

// waitUntilCryptohomeInit is a helper function to wait until cryptohome initialized.
func waitUntilCryptohomeInit(ctx context.Context, remote *hwsecremote.CmdRunnerRemote) error {
	return testing.Poll(ctx, func(context.Context) error {
		file := filepath.Join(resultDir, cryptohomeDbusTimeFile)
		_, err := remote.Run(ctx, "stat", file)
		if err != nil {
			return errors.Wrap(err, "stat file failed")
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: time.Second,
	})
}

// waitUntilUserdataauthInit is a helper function to wait until userdataauth initialized.
func waitUntilUserdataauthInit(ctx context.Context, remote *hwsecremote.CmdRunnerRemote) error {
	return testing.Poll(ctx, func(context.Context) error {
		file := filepath.Join(resultDir, userdataauthDbusTimeFile)
		_, err := remote.Run(ctx, "stat", file)
		if err != nil {
			return errors.Wrap(err, "stat file failed")
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: time.Second,
	})
}

// getTime returns timestamp for a cryptohome event.
func getTime(ctx context.Context, remote *hwsecremote.CmdRunnerRemote, eventName string) (float64, error) {
	eventFile := filepath.Join(resultDir, eventName)
	data, err := remote.Run(ctx, "cat", eventFile)
	if err != nil {
		return 0.0, err
	}
	trimmedData := strings.TrimSpace(string(data))

	stamp, err := strconv.ParseFloat(trimmedData, 64)
	if err != nil {
		return 0.0, err
	}

	// From nano second to second
	stamp = stamp / 1e9

	return stamp, nil
}

func clearOwnership(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewFullHelper(r, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	attestation := helper.AttestationClient()

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	if result, err := attestation.IsPreparedForEnrollment(ctx); err != nil {
		s.Fatal("Cannot check if enrollment preparation is reset: ", err)
	} else if result {
		s.Fatal("Enrollment preparation is not reset after clearing ownership")
	}
	s.Log("Enrolling with TPM not ready")
	if _, err := attestation.CreateEnrollRequest(ctx, hwsec.DefaultPCA); err == nil {
		s.Fatal("Enrollment should not happen w/o getting prepared")
	}
}

func CryptohomePerf(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())

	clearOwnership(ctx, s)

	err := waitUntilCryptohomeInit(ctx, r)
	if err != nil {
		s.Fatal("Failed to wait cryptohome init: ", err)
	}

	enableUserdataauth := false
	_, err = r.Run(ctx, "/usr/libexec/cryptohome/shall-use-userdataauth.sh")
	if err == nil {
		enableUserdataauth = true
	}

	if enableUserdataauth {
		err = waitUntilUserdataauthInit(ctx, r)
		if err != nil {
			s.Fatal("Failed to wait userdataauth init: ", err)
		}
	}

	cryptohomeDbusTime, err := getTime(ctx, r, cryptohomeDbusTimeFile)
	if err != nil {
		s.Fatal("Failed to parse cryptohome D-Bus startup time: ", err)
	}

	s.Log("start-up time of cryptohome D-Bus: ", cryptohomeDbusTime)

	var userdataauthDbusTime float64
	if enableUserdataauth {
		userdataauthDbusTime, err = getTime(ctx, r, userdataauthDbusTimeFile)
		if err != nil {
			s.Fatal("Failed to parse userdataauth D-Bus startup time: ", err)
		}
		s.Log("start-up time of userdataauth D-Bus: ", userdataauthDbusTime)
	}

	// Record the perf measurements.
	value := perf.NewValues()

	value.Set(perf.Metric{
		Name:      "crpytohome_start_time",
		Variant:   "cryptohome_dbus",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, cryptohomeDbusTime)

	if enableUserdataauth {
		value.Set(perf.Metric{
			Name:      "crpytohome_start_time",
			Variant:   "userdataauth_dbus",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}, userdataauthDbusTime)
	}

	value.Save(s.OutDir())
}
