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
	userDataAuthDbusTimeFile = "userdataauth_dbus_time"
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

// userDataAuthInitTime is a helper function to get userdataauth init time.
func userDataAuthInitTime(ctx context.Context, remote *hwsecremote.CmdRunnerRemote) (float64, error) {
	var result float64
	err := testing.Poll(ctx, func(context.Context) error {
		file := filepath.Join(resultDir, userDataAuthDbusTimeFile)
		data, err := remote.Run(ctx, "cat", file)
		if err != nil {
			return errors.Wrap(err, "failed to cat file")
		}
		trimmedData := strings.TrimSpace(string(data))

		stamp, err := strconv.ParseFloat(trimmedData, 64)
		if err != nil {
			return errors.Wrap(err, "failed to convert to float")
		}

		// From nano second to second
		result = stamp / 1e9

		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: time.Second,
	})
	return result, err
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

	userDataAuthDBusTime, err := userDataAuthInitTime(ctx, r)
	if err != nil {
		s.Fatal("Failed to get userdataauth D-Bus startup time: ", err)
	}

	// Record the perf measurements.
	value := perf.NewValues()

	value.Set(perf.Metric{
		Name:      "crpytohome_start_time",
		Variant:   "userdataauth_dbus",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, userDataAuthDBusTime)

	value.Save(s.OutDir())
}
