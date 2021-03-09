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

func CryptohomePerf(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	if err := helper.EnsureTPMIsResetAndPowerwash(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	err = waitUntilUserdataauthInit(ctx, r)
	if err != nil {
		s.Fatal("Failed to wait userdataauth init: ", err)
	}

	userdataauthDbusTime, err := getTime(ctx, r, userdataauthDbusTimeFile)
	if err != nil {
		s.Fatal("Failed to parse userdataauth D-Bus startup time: ", err)
	}
	s.Log("start-up time of userdataauth D-Bus: ", userdataauthDbusTime)

	// Record the perf measurements.
	value := perf.NewValues()

	value.Set(perf.Metric{
		Name:      "crpytohome_start_time",
		Variant:   "userdataauth_dbus",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, userdataauthDbusTime)

	value.Save(s.OutDir())
}
