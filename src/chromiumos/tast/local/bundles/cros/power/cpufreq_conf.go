// Copyright 2021 The ChromiumOS Authors
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
		Attr: []string{"group:mainline"},
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
				return errors.Errorf("unexpected governor: got %q, want %q", governor, expectedGovernor)
			}
		}

		return nil
	}

	testEPP := func(expectedEPP string) error {
		paths, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/energy_performance_preference")
		if err != nil {
			return errors.Wrap(err, "failed to glob for EPP settings")
		}

		for _, path := range paths {
			out, err := ioutil.ReadFile(path)
			if err != nil {
				return errors.Wrap(err, "failed to read energy performance preference")
			}

			epp := strings.TrimSpace(string(out))
			if epp != expectedEPP {
				return errors.Errorf("unexpected EPP: got %q, want %q", epp, expectedEPP)
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
			s.Logf("Charging status %q doesn't match charging=%t; not checking governor %q", status, charging, expectedGovernor)
			return nil
		}
		return testGovernor(expectedGovernor)
	}

	testGovernorSetting := func(governor, setting, expected string) error {
		// Look for both multi-policy (glob) and single-policy paths.
		paths, err := filepath.Glob(filepath.Join("/sys/devices/system/cpu/cpu[0-9]*/cpufreq", governor, setting))
		if err != nil {
			return errors.Wrap(err, "failed to glob for governor setting")
		}

		singlePath := filepath.Join("/sys/devices/system/cpu/cpufreq", governor, setting)
		if _, err := os.Stat(singlePath); !os.IsNotExist(err) {
			paths = append(paths, singlePath)
		}

		for _, path := range paths {
			out, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			val := strings.TrimSpace(string(out))
			if val != expected {
				return errors.Errorf("unexpected setting for governor %q (%s): got %q, want %q", governor, path, val, expected)
			}
		}

		return nil
	}

	keyTests := map[string]func(string) error{
		"CPUFREQ_GOVERNOR": testGovernor,
		"CPUFREQ_GOVERNOR_BATTERY_CHARGE": func(expectedGovernor string) error {
			return testChargeGovernor(true /* charging */, expectedGovernor)
		},
		"CPUFREQ_GOVERNOR_BATTERY_DISCHARGE": func(expectedGovernor string) error {
			return testChargeGovernor(false /* charging */, expectedGovernor)
		},
		"CPUFREQ_ENERGY_PERFORMANCE_PREFERENCE": testEPP,
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
		"conservative": []string{
			"freq_step",
			"down_threshold",
		},
		"schedutil": []string{
			"rate_limit_us",
		},
	}
	for gov, settings := range governorSettings {
		for _, setting := range settings {
			settingKey := "CPUFREQ_" + strings.ToUpper(setting)
			genTest := func(gov, setting string) func(string) error {
				return func(expectedSetting string) error {
					return testGovernorSetting(gov, setting, expectedSetting)
				}
			}
			keyTests[settingKey] = genTest(gov, setting)
		}
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
