// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TimeTakenSuspendResume,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Time Taken to suspend and resume for S0ix",
		Contacts: []string{
			"ambalavanan.m.m@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		SoftwareDeps: []string{"chrome", "pmc_cstate_show"},
		HardwareDeps: hwdep.D(hwdep.X86()),
		Vars: []string{
			"power.TimeTakenSuspendResume.defaultSuspendTime",
			"power.TimeTakenSuspendResume.defaultResumeTime",
		},
		Fixture: "chromeLoggedIn",
		Timeout: 5 * time.Minute,
	})
}

func TimeTakenSuspendResume(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	const (
		slpS0Cmd           = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateCmd       = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
		powerdConfigCmd    = "check_powerd_config --suspend_to_idle; echo $?"
		defaultSuspendTime = 0.5 // default suspend time in seconds.
		defaultResumeTime  = 0.5 // default resume time in seconds.
	)

	// timeVar returns the suspend/resume time in Milliseconds.
	timeVar := func(name string, defaultValue float64) time.Duration {
		str, ok := s.Var(name)
		if !ok {
			return time.Duration(defaultValue*1000) * time.Millisecond
		}

		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			s.Fatalf("Failed to parse float64 variable %v: %v", name, err)
		}

		return time.Duration(val*1000) * time.Millisecond
	}

	suspendTime := timeVar("power.TimeTakenSuspendResume.defaultSuspendTime", defaultSuspendTime)
	resumeTime := timeVar("power.TimeTakenSuspendResume.defaultResumeTime", defaultResumeTime)

	cmdOutput := func(cmd string) string {
		s.Logf("Executing command: %s", cmd)
		out, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", cmd, err)
		}
		return string(out)
	}

	earliestResumeEndTime := time.Now()

	configValue, err := testexec.CommandContext(ctx, "bash", "-c", powerdConfigCmd).
		Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to execute %s command: %v", powerdConfigCmd, err)
	}
	actualValue := strings.TrimSpace(string(configValue))
	const expectedValue = "0"
	if actualValue != expectedValue {
		s.Fatalf(
			"Failed to be in S0ix state; expected PowerdConfig value %s; got %s",
			expectedValue,
			actualValue,
		)
	}

	slpOpSetPre := cmdOutput(slpS0Cmd)
	pkgOpSetOutput := cmdOutput(pkgCstateCmd)
	c10PkgPattern := regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	matchSetPre := c10PkgPattern.FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}
	pkgOpSetPre := matchSetPre[1]

	if err := suspendAndResume(ctx, cr); err != nil {
		s.Fatal("Failed to suspend resume the DUT: ", err)
	}

	slpOpSetPost := cmdOutput(slpS0Cmd)
	if slpOpSetPre == slpOpSetPost {
		s.Fatalf(
			"SLP counter value %q must be different than the value noted most recently %q",
			slpOpSetPre,
			slpOpSetPost,
		)
	}
	if slpOpSetPost == "0" {
		s.Fatal("SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}
	pkgOpSetPostOutput := cmdOutput(pkgCstateCmd)
	matchSetPost := c10PkgPattern.FindStringSubmatch(pkgOpSetPostOutput)
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}
	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf(
			"Package C10 value %q must be different than value noted most recently %q",
			pkgOpSetPre,
			pkgOpSetPost,
		)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Package C10 should be non-zero, but got: ", pkgOpSetPost)
	}

	pv := perf.NewValues()

	suspendDuration, resumeDuration, err := readSuspendResumeDuration(ctx, earliestResumeEndTime)
	if err != nil {
		s.Fatal("Failed to read wakeup time: ", err)
	}

	sd := time.Duration(suspendDuration*1000) * time.Millisecond
	s.Log("Suspend time: ", sd)
	pv.Set(perf.Metric{
		Name:      "TimeTakenSuspendResume.SuspendTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(sd.Milliseconds()))

	rd := time.Duration(resumeDuration*1000) * time.Millisecond
	s.Log("Resume time: ", rd)
	pv.Set(perf.Metric{
		Name:      "TimeTakenSuspendResume.ResumeTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(rd.Milliseconds()))
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}

	if sd > suspendTime || rd > resumeTime {
		s.Fatalf(
			"Failed : Suspend or Resume duration is greater than expected; got suspend time %v, want suspend time %v, got resume time %v, want resume time %v",
			sd,
			suspendTime,
			rd,
			resumeTime,
		)
	}
}

// suspendAndResume calls powerd_dbus_suspend command to suspend the system
// and lets it stay sleep for the given duration and then wake up.
func suspendAndResume(ctx context.Context, cr *chrome.Chrome) error {
	const timeout = 30

	// Read wakeup count here to prevent suspend retries, which happens without
	// user input.
	wakeupCount, err := ioutil.ReadFile("/sys/power/wakeup_count")
	if err != nil {
		return errors.Wrap(err, "failed to read wakeup count before suspend")
	}

	cmd := testexec.CommandContext(
		ctx,
		"powerd_dbus_suspend",
		"--delay=0",
		fmt.Sprintf("--timeout=%d", timeout),
		fmt.Sprintf("--wakeup_count=%s", strings.Trim(string(wakeupCount), "\n")),
		"--wakeup_timeout=15",
	)
	testing.ContextLogf(ctx, "Suspend DUT for %d seconds: %s", timeout, cmd.Args)

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "powerd_dbus_suspend failed to properly suspend")
	}

	testing.ContextLog(ctx, "DUT resumes from suspend")
	return cr.Reconnect(ctx)
}

// readSuspendResumeDuration reads and calculates the wakeup duration from
// last_resume_timings file.
// The file's modification time must be newer than the earliestModTime to
// ensure the file has been updated by a successful suspend/wakeup.
func readSuspendResumeDuration(
	ctx context.Context,
	earliestModTime time.Time,
) (float64, float64, error) {
	const (
		lastResumeTimingsFile = "/run/power_manager/root/last_resume_timings"

		// suspendTotalTime is the time used to wait for suspend procedure to
		// generate the last_resume_timings file. In case of suspending failure,
		// the DUT might retry multiple times until it succeeds.
		suspendTotalTime = 2 * time.Minute
	)

	// Wait until the suspend procedure successfully generates the last_resume_timings
	// with a newer timestamp.
	pollOpts := testing.PollOptions{Timeout: suspendTotalTime, Interval: time.Second}
	if err := testing.Poll(ctx, func(c context.Context) error {
		fState, err := os.Stat(lastResumeTimingsFile)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("file doesn't exist")
			}
			return testing.PollBreak(errors.Wrap(err, "failed to check file state"))
		}
		if !fState.ModTime().After(earliestModTime) {
			return errors.New("last_resume_timings file hasn't been updated")
		}
		return nil
	}, &pollOpts); err != nil {
		return 0.0, 0.0, errors.Wrapf(
			err,
			"failed to check existence of a new last_resume_timings file within %v",
			pollOpts.Timeout,
		)
	}

	b, err := ioutil.ReadFile(lastResumeTimingsFile)
	if err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to read last_resume_timings file")
	}

	// The content of /run/power_manager/root/last_resume_timings should be as follows:
	// start_suspend_time = 183.825542
	// end_suspend_time = 184.213222
	// start_resume_time = 184.248745
	// end_resume_time = 185.480335
	// cpu_ready_time = 184.837355
	startResumeTimeExp := regexp.MustCompile(`start_resume_time\s*=\s*(\d+\.\d+)`)
	endResumeTimeExp := regexp.MustCompile(`end_resume_time\s*=\s*(\d+\.\d+)`)
	startSuspendTimeExp := regexp.MustCompile(`start_suspend_time\s*=\s*(\d+\.\d+)`)
	endSuspendTimeExp := regexp.MustCompile(`end_suspend_time\s*=\s*(\d+\.\d+)`)

	startResumeTime := startResumeTimeExp.FindStringSubmatch(string(b))
	endResumeTime := endResumeTimeExp.FindStringSubmatch(string(b))
	startSuspendTime := startSuspendTimeExp.FindStringSubmatch(string(b))
	endSuspendTime := endSuspendTimeExp.FindStringSubmatch(string(b))

	startResumeTimestamp, err := strconv.ParseFloat(startResumeTime[1], 64)
	if err != nil {
		return 0.0, 0.0, errors.Wrapf(err, "failed to get timestamp for %v", startResumeTime)
	}

	endResumeTimestamp, err := strconv.ParseFloat(endResumeTime[1], 64)
	if err != nil {
		return 0.0, 0.0, errors.Wrapf(err, "failed to get timestamp for %v", endResumeTime)
	}

	startSuspendTimestamp, err := strconv.ParseFloat(startSuspendTime[1], 64)
	if err != nil {
		return 0.0, 0.0, errors.Wrapf(err, "failed to get timestamp for %v", startSuspendTime)
	}

	endSuspendTimestamp, err := strconv.ParseFloat(endSuspendTime[1], 64)
	if err != nil {
		return 0.0, 0.0, errors.Wrapf(err, "failed to get timestamp for %v", endSuspendTime)
	}

	secondsSystemSuspend := endSuspendTimestamp - startSuspendTimestamp
	secondSystemResume := endResumeTimestamp - startResumeTimestamp

	return secondsSystemSuspend, secondSystemResume, nil
}
