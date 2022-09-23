// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Idle,
		Desc: "Verify that graphics behaves as expected on idle",
		Attr: []string{"group:mainline", "informational", "group:graphics", "graphics_nightly"},
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name: "dvfs",
			Val:  dvfs,
		}, {
			Name:              "fbc",
			Val:               fbc,
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "psr",
			Val:               psr,
			ExtraHardwareDeps: hwdep.D(hwdep.X86()),
		}, {
			Name:              "gem_idle",
			Val:               gemIdle,
			ExtraHardwareDeps: hwdep.D(hwdep.X86()),
		}, {
			Name:              "i915_min_clock",
			Val:               i915MinClock,
			ExtraHardwareDeps: hwdep.D(hwdep.X86()),
		}, {
			Name:              "rc6",
			Val:               rc6,
			ExtraHardwareDeps: hwdep.D(hwdep.X86()),
		}},
		Fixture: "chromeGraphics",
	})
}

func Idle(ctx context.Context, s *testing.State) {
	if err := cpu.WaitUntilStabilized(ctx, cpu.CoolDownConfig{
		PollTimeout:              2 * time.Minute,
		PollInterval:             2 * time.Second,
		TemperatureThresholdMode: cpu.TemperatureThresholdPerModel,
		TemperatureThreshold:     46000,
		CoolDownMode:             cpu.CoolDownPreserveUI,
	}); err != nil {
		s.Log("Failed to get stable CPU before running tests: ", err)
	}
	check := s.Param().(func(context.Context) error)
	if err := check(ctx); err != nil {
		s.Error("Failed with: ", err)
	}
}

// getValidDir search the list of paths and return the directory which exists.
func getValidDir(paths []string) (string, error) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		return path, nil
	}
	return "", errors.Errorf("none of %v exist", paths)
}

// getValidPath search the list of paths and return the path which exists.
func getValidPath(paths []string) (string, error) {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		return path, nil
	}
	return "", errors.Errorf("none of %v exist", paths)
}

// dvfs check that we get into the lowest clock frequency on system which support DVFS.
func dvfs(ctx context.Context) error {
	node, err := getValidDir([]string{
		// MT8173
		"/sys/devices/soc/13000000.mfgsys-gpu/",
		// MT8173_419
		"/sys/devices/platform/soc/13000000.mfgsys-gpu/",
		// MT8183
		"/sys/devices/platform/soc/13040000.mali/",
		// MT8192
		"/sys/devices/platform/soc/13000000.mali/",
	})
	if err != nil {
		return errors.Wrap(err, "unknown soc for dvfs")
	}
	matches, err := filepath.Glob(filepath.Join(node, "devfreq", "*"))
	if err != nil {
		return errors.Wrapf(err, "failed to glob devfreq device under %v", node)
	}
	if len(matches) != 1 {
		return errors.Wrapf(err, "expect 1 devfreq device, got %v", matches)
	}
	devFreqPath := matches[0]
	governorPath := filepath.Join(devFreqPath, "governor")
	clockPath := filepath.Join(devFreqPath, "cur_freq")

	out, err := os.ReadFile(governorPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read dvfs governor path %v", governorPath)
	}
	governors := strings.Split(string(out), "\n")
	if len(governors) != 1 {
		return errors.Wrapf(err, "more than 1 governor found: %v", governors)
	}
	governor := governors[0]
	testing.ContextLogf(ctx, "DVFS governor = %s", governor)
	if governor != "simple_ondemand" {
		return errors.Errorf("expect simple_ondemand dvfs governor, got %v", governor)
	}

	frequenciesPath := filepath.Join(devFreqPath, "available_frequencies")
	// available_frequencies are always sorted in ascending order.
	// each line may contain one or multiple integers separated by spaces.
	out, err = os.ReadFile(frequenciesPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", frequenciesPath)
	}
	frequencies := strings.Split(string(out), "\n")
	minFreq, err := strconv.ParseInt(frequencies[0], 0, 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse frequency %v", frequencies[0])
	}
	testing.ContextLog(ctx, "Expecting idle DVFS clock: ", minFreq)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		f, err := os.ReadFile(clockPath)
		if err != nil {
			return errors.Wrapf(err, "failed to open %v", clockPath)
		}
		clockStr := strings.Split(string(f), "\n")[0]
		clock, err := strconv.ParseInt(clockStr, 0, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse clock %v", clockStr)
		}
		if clock > minFreq {
			return errors.Errorf("clock frequency %v is higher than idle DVFS clock %v", clock, minFreq)
		}
		testing.ContextLog(ctx, "Found idle DVFS clock: ", clock)
		return nil
	}, &testing.PollOptions{
		Timeout: 1 * time.Minute,
	}); err != nil {
		return err
	}
	return nil
}

// fbc check that we can get into FBC.
func fbc(ctx context.Context) error {
	fbcPaths := []string{
		"/sys/kernel/debug/dri/0/i915_fbc_status",
	}
	fbcPath, err := getValidPath(fbcPaths)
	if err != nil {
		return errors.Wrap(err, "no FBC_PATHS found")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		f, err := os.ReadFile(fbcPath)
		if err != nil {
			return errors.Wrapf(err, "failed to open %v", fbcPath)
		}
		re := regexp.MustCompile("FBC enabled")
		matches := re.FindStringSubmatch(string(f))
		if matches == nil {
			return errors.Wrapf(err, "FBC enabled not found, last content is %v", string(f))
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 1 * time.Minute,
	}); err != nil {
		return err
	}
	return nil
}

// psr check that we can get into PSR.
func psr(ctx context.Context) error {
	psrPaths := []string{"/sys/kernel/debug/dri/0/i915_edp_psr_status"}
	psrPath, err := getValidPath(psrPaths)
	if err != nil {
		return errors.Wrap(err, "failed to found valid psr path")
	}
	kernelVersion, _, err := sysutil.KernelVersionAndArch()
	if err != nil {
		return errors.Wrap(err, "failed to get kernel version")
	}
	testing.ContextLogf(ctx, "Kernel version: %s", kernelVersion)

	// Check if PSR is enabled on the device.
	f, err := os.ReadFile(psrPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", psrPath)
	}
	re := regexp.MustCompile("Enabled: yes")
	matches := re.FindStringSubmatch(string(f))
	if matches == nil {
		testing.ContextLog(ctx, "PSR not enabled")
		return nil
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		f, err := os.ReadFile(psrPath)
		if err != nil {
			return errors.Wrapf(err, "failed to open %v", psrPath)
		}
		var re *regexp.Regexp
		if kernelVersion.IsOrLater(4, 4) {
			re = regexp.MustCompile("PSR status: .* \\[SRDENT")
		} else if kernelVersion.Is(3, 18) {
			re = regexp.MustCompile("Performance_Counter: 0")
		}
		if re.FindStringSubmatch(string(f)) == nil {
			return errors.Errorf("didn't not see psr activity in %s", psrPath)
		}
		testing.ContextLogf(ctx, "Found active with kernel: %s", kernelVersion)
		return nil
	}, &testing.PollOptions{
		Timeout: 1 * time.Minute,
	}); err != nil {
		return err
	}
	return nil
}

// gemIdle check that we can get all gem objects to become idle.
func gemIdle(ctx context.Context) error {
	kernelVersion, _, err := sysutil.KernelVersionAndArch()
	if err != nil {
		return errors.Wrap(err, "failed to get kernel version")
	}

	if kernelVersion.IsOrLater(5, 10) {
		// The data needed for this test was removed in the 5.10 kernel.
		// See b/179453336 for details.
		testing.ContextLog(ctx, "Skipping gem idle check on kernel 5.10 and above")
		return nil
	}

	perProcessCheck := false
	gemPath, err := getValidPath([]string{"/sys/kernel/debug/dri/0/i915_gem_active"})
	if err != nil {
		gemPath, err = getValidPath([]string{"/sys/kernel/debug/dri/0/i915_gem_objects"})
		if err != nil {
			return errors.Wrap(err, "no gem path is found")
		}
		perProcessCheck = true
	}

	var pollFunc func(context.Context) error
	if perProcessCheck {
		// Check 4.4 and later kernels
		pollFunc = func(ctx context.Context) error {
			f, err := os.ReadFile(gemPath)
			if err != nil {
				return errors.Wrapf(err, "failed to open %v", gemPath)
			}

			re := regexp.MustCompile("\n.*\\(0 active,")
			if re.FindStringSubmatch(string(f)) == nil {
				return errors.Errorf("can't find 0 gem activities in %v", gemPath)
			}
			return nil
		}
	} else {
		// Check pre 4.4 kernels
		pollFunc = func(ctx context.Context) error {
			f, err := os.ReadFile(gemPath)
			if err != nil {
				return errors.Wrapf(err, "failed to open %v", gemPath)
			}
			re := regexp.MustCompile("Total 0 objects")
			if re.FindStringSubmatch(string(f)) == nil {
				return errors.Errorf("can't find 0 gem activities in %v", gemPath)
			}
			return nil
		}
	}
	if err := testing.Poll(ctx, pollFunc, &testing.PollOptions{
		Timeout: 1 * time.Minute,
	}); err != nil {
		return err
	}
	return nil
}

// i915MinClock check that we get into the lowest clock frequency.
func i915MinClock(ctx context.Context) error {
	clockPath, err := getValidPath([]string{
		"/sys/kernel/debug/dri/0/i915_frequency_info",
		"/sys/kernel/debug/dri/0/i915_cur_delayinfo",
	})
	if err != nil {
		return errors.Wrap(err, "failed to get clock path")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		f, err := os.ReadFile(clockPath)
		if err != nil {
			return errors.Wrapf(err, "failed to open %v", clockPath)
		}

		// This file has a different format depending on the
		// board, so we parse both. Also, it would be tedious
		// to add the minimum clock for each board, so instead
		// we use 650MHz which is the max of the minimum clocks.
		re := regexp.MustCompile("CAGF: (.*)MHz")
		matches := re.FindStringSubmatch(string(f))
		if matches != nil {
			hz, err := strconv.ParseInt(matches[1], 0, 64)
			if err != nil {
				return errors.Wrapf(err, "failed to parse %s to int", matches[1])
			}
			if hz <= 650 {
				return nil
			}
		}

		re = regexp.MustCompile("current GPU freq: (.*) MHz")
		matches = re.FindStringSubmatch(string(f))
		if matches != nil {
			hz, err := strconv.ParseInt(matches[1], 0, 64)
			if err != nil {
				return errors.Wrapf(err, "failed to parse %s to int", matches[1])
			}
			if hz <= 650 {
				return nil
			}
		}
		return errors.New("did not see the min i915 clock")

	}, &testing.PollOptions{
		Timeout: 1 * time.Minute,
	}); err != nil {
		return err
	}
	return nil
}

// rc6 check that we are able to get into rc6 on systems which support RC6 (non atom).
func rc6(ctx context.Context) error {
	rc6Path, err := getValidPath([]string{
		"/sys/kernel/debug/dri/0/i915_drpc_info",
		"/sys/kernel/debug/dri/0/gt/drpc",
	})
	if err != nil {
		return errors.Wrap(err, "failed to get valid rc6 path")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		f, err := os.ReadFile(rc6Path)
		if err != nil {
			return errors.Wrapf(err, "failed to open %v", rc6Path)
		}
		re := regexp.MustCompile("Current RC state: (.*)")
		matches := re.FindStringSubmatch(string(f))
		if matches != nil && matches[1] == "RC6" {
			return nil
		}
		return errors.New("did not see the GPU in RC6")
	}, &testing.PollOptions{
		Timeout: 1 * time.Minute,
	}); err != nil {
		return err
	}
	return nil
}
