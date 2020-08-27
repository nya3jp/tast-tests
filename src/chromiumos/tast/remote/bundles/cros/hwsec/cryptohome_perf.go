// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"math"
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
	bootstatDir    = "/tmp"
	uptimePrefix   = "uptime-"
	cryptohomeName = "cryptohome"
	tpmManagerName = "tpm_manager"
	chapsName      = "chaps"
	startedSuffix  = "-started"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomePerf,
		Desc: "Cryptohome performance test",
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts: []string{
			"yich@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"reboot", "tpm"},
	})
}

// waitUntilBootComplete is a helper function to wait until boot complete and
// we are ready to collect boot metrics.
func waitUntilBootComplete(ctx context.Context, remote *hwsecremote.CmdRunnerRemote) error {
	return testing.Poll(ctx, func(context.Context) error {
		// Check that bootstat files are available.
		for _, services := range []string{cryptohomeName, tpmManagerName, chapsName} {
			file := filepath.Join(bootstatDir, uptimePrefix+services+startedSuffix)
			data, err := remote.Run(ctx, "stat", file)
			if err != nil {
				return errors.Wrapf(err, "stat file failed %q", data)
			}
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: time.Second,
	})
}

// parseBootstat reads values from a bootstat event file. Each line of a
// bootstat event file represents one occurrence of the event. Each line is a
// copy of the content of /proc/uptime ("uptime-" files) , captured at the time of the
// occurrence. Each line is a blank separated list of
// fields. This function reads all lines (occurrences) in the event file, and returns the
// value of the given field.
func parseBootstat(ctx context.Context, remote *hwsecremote.CmdRunnerRemote, fileName string, fieldNum int) ([]float64, error) {
	var result []float64
	b, err := remote.Run(ctx, "cat", fileName)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, errors.Errorf("bootstat file %s is empty", fileName)
	}

	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		f := strings.Fields(line)
		if fieldNum >= len(f) {
			continue
		}
		s, err := strconv.ParseFloat(f[fieldNum], 64)
		if err != nil {
			return nil, errors.Wrapf(err, "malformed bootstat content: %s", line)
		}
		result = append(result, s)
	}

	return result, nil
}

// parseUptime returns time since boot for a bootstat event.
func parseUptime(ctx context.Context, remote *hwsecremote.CmdRunnerRemote, eventName string, index int) (float64, error) {
	eventFile := filepath.Join(bootstatDir, uptimePrefix+eventName+startedSuffix)
	val, err := parseBootstat(ctx, remote, eventFile, 0)
	if err != nil {
		return 0.0, err
	}

	n := len(val)
	// Check for OOB access.
	if index < -n || index > n-1 {
		return 0.0, errors.Errorf("bootstat index out of bound. len=%d, index=%d", n, index)
	}

	if index >= 0 {
		return val[index], nil
	}
	// Like negative index in python.
	return val[n+index], nil
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
		s.Fatal("CmdRunner creation error: ", err)
	}
	clearOwnership(ctx, s)
	err = waitUntilBootComplete(ctx, r)
	if err != nil {
		s.Fatal("WaitUntilBootComplete error: ", err)
	}
	cryptohomeUptime, err := parseUptime(ctx, r, cryptohomeName, 0)
	if err != nil {
		s.Fatal("parseUptime error: ", err)
	}
	tpmManagerUptime, err := parseUptime(ctx, r, tpmManagerName, 0)
	if err != nil {
		s.Fatal("parseUptime error: ", err)
	}
	chapsUptime, err := parseUptime(ctx, r, chapsName, 0)
	if err != nil {
		s.Fatal("parseUptime error: ", err)
	}
	// Record the perf measurements.
	value := perf.NewValues()

	startUpTime := math.Min(cryptohomeUptime-tpmManagerUptime, cryptohomeUptime-chapsUptime)

	s.Log("start-up time of cryptohome: ", startUpTime)

	value.Set(perf.Metric{
		Name:      "crpytohome_start_time",
		Variant:   "dbus",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false,
	}, startUpTime)

}
