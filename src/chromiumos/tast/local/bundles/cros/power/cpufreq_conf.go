// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	localpower "chromiumos/tast/local/power"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CpufreqConf,
		Desc: "Check that we respect the /etc/cpufreq.conf file",
		Contacts: []string{
			"briannorris@chromium.org",
			"chromeos-platform-power@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func CpufreqConf(ctx context.Context, s *testing.State) {
	parseConf := func(conf string) map[string]string {
		m := make(map[string]string)
		for _, l := range strings.Split(conf, "\n") {
			pair := strings.SplitN(l, "=", 2)
			if len(pair) < 2 {
				continue
			}
			// Configuration files use shell-like quoting.
			m[pair[0]] = strings.Trim(pair[1], "\"")
		}
		return m
	}

	testGovernor := func(expectedGovernor string) error {
		paths, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor")
		if err != nil {
			return errors.Wrap(err, "failed to glob for governors")
		}

		for _, path := range paths {
			out, err := ioutil.ReadFile(path)
			if err != nil {
				return errors.Wrap(err, "failed to read governor")
			}

			governor := strings.TrimSpace(string(out))
			if governor != expectedGovernor {
				return errors.Errorf("unexpected governor: %q != %q", governor, expectedGovernor)
			}
		}

		return nil
	}

	testChargeGovernor := func(charging bool, expectedGovernor string) error {
		batteryPath, err := localpower.SysfsBatteryPath(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get battery path")
		}
		status, err := localpower.ReadBatteryStatus(batteryPath)
		if err != nil {
			return errors.Wrap(err, "failed to read battery status")
		}
		if (status != localpower.BatteryStatusDischarging) == charging {
			s.Logf("Charging status %q doesn't match charging=%t", status, charging)
			return nil
		}
		return testGovernor(expectedGovernor)
	}

	out, err := ioutil.ReadFile("/etc/cpufreq.conf")
	if os.IsNotExist(err) {
		s.Log("No cpufreq.conf file")
		return
	}
	if err != nil {
		s.Fatal("Failed to read cpufreq.conf: ", err)
	}

	conf := parseConf(string(out))
	s.Log(conf)

	// Force cpufreq job, in case previous tests modified the governor settings and didn't
	// clean up.
	if err := upstart.RestartJob(ctx, "cpufreq"); err != nil {
		s.Fatal("Failed to run cpufreq job: ", err)
	}

	for _, keyTest := range []struct {
		key  string
		test func(string) error
	}{
		{
			"CPUFREQ_GOVERNOR",
			testGovernor,
		}, {
			"CPUFREQ_GOVERNOR_BATTERY_CHARGE",
			func(expectedGovernor string) error {
				return testChargeGovernor(true /* charging */, expectedGovernor)
			},
		}, {
			"CPUFREQ_GOVERNOR_BATTERY_DISCHARGE",
			func(expectedGovernor string) error {
				return testChargeGovernor(false /* charging */, expectedGovernor)
			},
		},
	} {
		val, ok := conf[keyTest.key]
		if !ok {
			continue
		}

		if err := keyTest.test(val); err != nil {
			s.Errorf("Key %q, value %q failed: %v", keyTest.key, val, err)
		}
	}
}
