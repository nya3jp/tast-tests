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
		// Future work (?): simulate AC on/off, to test all paths.
		if (status != localpower.BatteryStatusDischarging) == charging {
			s.Logf("Charging status %q doesn't match charging=%t", status, charging)
			return nil
		}
		return testGovernor(expectedGovernor)
	}

	testGovernorSetting := func(governor, setting, expected string) error {
		path := filepath.Join("/sys/devices/system/cpu/cpufreq", governor, setting)
		out, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		val := strings.TrimSpace(string(out))
		if val != expected {
			return errors.Errorf("unexpected setting for governor %q: %q != %q", governor, val, expected)
		}

		return nil
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
	s.Log("cpufreq.conf contents: ", conf)

	// Force cpufreq job, in case previous tests modified the governor settings and didn't
	// clean up.
	if err := upstart.RestartJob(ctx, "cpufreq"); err != nil {
		s.Fatal("Failed to run cpufreq job: ", err)
	}

	keyTests := map[string]func(string) error{
		"CPUFREQ_GOVERNOR": testGovernor,
		"CPUFREQ_GOVERNOR_BATTERY_CHARGE": func(expectedGovernor string) error {
			return testChargeGovernor(true /* charging */, expectedGovernor)
		},
		"CPUFREQ_GOVERNOR_BATTERY_DISCHARGE": func(expectedGovernor string) error {
			return testChargeGovernor(false /* charging */, expectedGovernor)
		},
	}

	// Construct test map for governor-specific settings.
	governorSettings := map[string][]string{
		"interactive": []string{
			"input_boost",
			"above_hispeed_delay",
			"go_hispeed_load",
			"hispeed_freq",
			"min_sample_time",
			"target_loads",
			"timer_rate",
		},
		"ondemand": []string{
			"sampling_rate",
			"up_threshold",
			"ignore_nice_load",
			"io_is_busy",
			"sampling_down_factor",
			"powersave_bias",
		},
	}
	for gov, settings := range governorSettings {
		for _, setting := range settings {
			settingKey := "CPUFREQ_" + strings.ToUpper(setting)
			keyTests[settingKey] = func(expectedSetting string) error {
				return testGovernorSetting(gov, setting, expectedSetting)
			}
		}
	}

	for key, val := range conf {
		s.Logf("Checking configuration key %q, value %q", key, val)
		keyTest, ok := keyTests[key]
		if !ok {
			s.Errorf("Unexpected key %q in cpufreq.conf (value %q)", key, val)
			continue
		}

		if err := keyTest(val); err != nil {
			s.Errorf("Key %q, value %q failed: %v", key, val, err)
		}
	}
}
