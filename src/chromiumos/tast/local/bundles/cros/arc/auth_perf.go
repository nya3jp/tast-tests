// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

type testParam struct {
	username string
	password string
	// maxErrorBootCount is the number of maximum allowed boot errors.
	maxErrorBootCount int
	resultSuffix      string
	chromeArgs        []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthPerf,
		Desc: "Measure auth times in ARC",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"niwa@chromium.org",  // Tast port author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		// This test steps through opt-in flow 10 times and each iteration takes 20~40 seconds.
		Timeout: 20 * time.Minute,
		Params: []testing.Param{{
			Name:              "unmanaged",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				username:          "arc.AuthPerf.unmanaged_username",
				password:          "arc.AuthPerf.unmanaged_password",
				maxErrorBootCount: 1,
				resultSuffix:      "",
			},
		}, {
			Name:              "unmanaged_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				username:          "arc.AuthPerf.unmanaged_username",
				password:          "arc.AuthPerf.unmanaged_password",
				maxErrorBootCount: 3,
				resultSuffix:      "",
			},
		}, {
			Name:              "managed",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				username:          "arc.AuthPerf.managed_username",
				password:          "arc.AuthPerf.managed_password",
				maxErrorBootCount: 1,
				resultSuffix:      "_managed",
			},
		}, {
			Name:              "managed_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				username:          "arc.AuthPerf.managed_username",
				password:          "arc.AuthPerf.managed_password",
				maxErrorBootCount: 3,
				resultSuffix:      "_managed",
			},
		}},
		Vars: []string{
			"arc.AuthPerf.unmanaged_username",
			"arc.AuthPerf.unmanaged_password",
			"arc.AuthPerf.managed_username",
			"arc.AuthPerf.managed_password",
		},
	})
}

// measuredValues stores measured times in milliseconds.
type measuredValues struct {
	playStoreShownTime float64
	accountCheckTime   float64
	checkinTime        float64
	networkWaitTime    float64
	signInTime         float64
	energyUsage        *power.RAPLValues
}

// AuthPerf steps through multiple opt-ins and collects checkin time, network wait time,
// sign-in time and Play Store shown time.
// It also reports average, min and max results.
func AuthPerf(ctx context.Context, s *testing.State) {
	const (
		// successBootCount is the number of passing ARC boots to collect results.
		successBootCount = 10
	)

	param := s.Param().(testParam)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)
	maxErrorBootCount := param.maxErrorBootCount

	args := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui")

	// TODO(crbug.com/995869): Remove set of flags to disable app sync, PAI, locale sync, Play Store auto-update.
	cr, err := chrome.New(ctx, chrome.ARCSupported(), chrome.RestrictARCCPU(), chrome.GAIALogin(),
		chrome.Auth(username, password, ""),
		chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer tconn.Close()

	errorCount := 0
	var playStoreShownTimes []float64
	var accountCheckTimes []float64
	var checkinTimes []float64
	var networkWaitTimes []float64
	var signInTimes []float64
	var energyUsage []*power.RAPLValues

	for len(playStoreShownTimes) < successBootCount {
		s.Logf("Running ARC opt-in iteration #%d out of %d",
			len(playStoreShownTimes)+1, successBootCount)

		v, err := bootARC(ctx, s, cr, tconn)
		logcatName := ""
		if err == nil {
			// Append Play Store shown time in ms for quick reference.
			logcatName = fmt.Sprintf("logcat_ok_%d.log", int(v.playStoreShownTime))
		} else {
			logcatName = fmt.Sprintf("logcat_error_%d.log", errorCount)
		}
		logcatFilePath := filepath.Join(s.OutDir(), logcatName)
		if err := dumpLogcat(ctx, logcatFilePath); err != nil {
			s.Log("Failed to dump logcat: ", err)
		}
		if err != nil {
			errorCount++
			s.Logf("Error found during the ARC boot: %v - logcat was dumped to %s",
				err, logcatFilePath)

			if errorCount > maxErrorBootCount {
				s.Fatalf("Too many(%d) ARC boot errors", errorCount)
			}
			continue
		}

		playStoreShownTimes = append(playStoreShownTimes, v.playStoreShownTime)
		accountCheckTimes = append(accountCheckTimes, v.accountCheckTime)
		checkinTimes = append(checkinTimes, v.checkinTime)
		networkWaitTimes = append(networkWaitTimes, v.networkWaitTime)
		signInTimes = append(signInTimes, v.signInTime)
		if v.energyUsage != nil {
			energyUsage = append(energyUsage, v.energyUsage)
		}
	}

	perfValues := perf.NewValues()

	// Array for generating comma separated results.
	var resultForLog []string

	version, err := chromeOSVersion()
	if err != nil {
		s.Fatal("Can't get ChromeOS version: ", err)
	}
	resultForLog = append(resultForLog, version, strconv.Itoa(len(playStoreShownTimes)))

	reportResult := func(name string, samples []float64) {
		name = name + param.resultSuffix

		for _, x := range samples {
			perfValues.Append(perf.Metric{
				Name:      name,
				Unit:      "milliseconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, x)
		}

		var sum float64
		var max float64
		min := math.MaxFloat64
		for _, x := range samples {
			sum += x
			max = math.Max(x, max)
			min = math.Min(x, min)
		}
		average := sum / float64(len(samples))

		s.Logf("%s - average: %d, range %d - %d based on %d samples",
			name, int(average), int(min), int(max), len(samples))
		resultForLog = append(resultForLog, strconv.Itoa(int(min)),
			strconv.Itoa(int(average)), strconv.Itoa(int(max)))
	}

	reportResult("play_store_shown_time", playStoreShownTimes)
	reportResult("account_check_time", accountCheckTimes)
	reportResult("checkin_time", checkinTimes)
	reportResult("network_wait_time", networkWaitTimes)
	reportResult("sign_in_time", signInTimes)

	for _, rapl := range energyUsage {
		rapl.ReportPerfMetrics(perfValues, "power_")
	}

	// Outputs comma separated results to the log.
	// It is used in many sheet tables to automate paste operations.
	s.Log(strings.Join(resultForLog, ","))

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// bootARC performs one ARC boot iteration, opt-out and opt-in again.
// It calculates the time when the Play Store appears and set of ARC auth times.
func bootARC(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) (measuredValues, error) {
	var v measuredValues

	// Opt out.
	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		return v, err
	}

	s.Log("Waiting for Android data directory to be removed")
	if err := waitForAndroidDataRemoved(ctx); err != nil {
		return v, err
	}

	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	energyBefore, err := power.NewRAPLSnapshot()
	if err != nil {
		s.Error("Energy status is not available for this board")
	}

	startTime := time.Now()

	// Opt in. From performance perspective, optin longer than 90 seconds is failure.
	// This also aligned with global 20 minute timeout.
	shorterCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	s.Log("Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(shorterCtx, cr, tconn); err != nil {
		return v, err
	}

	s.Log("Waiting for Play Store window to be shown")
	if err := optin.WaitForPlayStoreShown(shorterCtx, tconn); err != nil {
		return v, err
	}

	v.playStoreShownTime = time.Now().Sub(startTime).Seconds() * 1000

	// energyBefore could be nil (not considered an error) on non-Intel CPUs.
	if energyBefore != nil {
		v.energyUsage, err = energyBefore.DiffWithCurrentRAPL()
		if err != nil {
			s.Fatal("Failed to get RAPL values: ", err)
		}
	}

	// Read sign-in results via property
	out, err := testexec.CommandContext(ctx,
		"android-sh", "-c", "getprop dev.arc.signin.result").Output(testexec.DumpLogOnError)
	if err != nil {
		return v, errors.Wrap(err, "failed to get signin result property")
	}

	outStr := string(out)
	m := regexp.MustCompile(`OK,(\d+),(\d+),(\d+),(\d+)`).FindStringSubmatch(outStr)
	if m == nil {
		return v, errors.Errorf("sign-in results could not be parsed: %q", outStr)
	}

	if v.signInTime, err = strconv.ParseFloat(m[1], 32); err != nil {
		return v, err
	}
	if v.accountCheckTime, err = strconv.ParseFloat(m[2], 32); err != nil {
		return v, err
	}
	if v.networkWaitTime, err = strconv.ParseFloat(m[3], 32); err != nil {
		return v, err
	}
	if v.checkinTime, err = strconv.ParseFloat(m[4], 32); err != nil {
		return v, err
	}

	return v, nil
}

// waitForAndroidDataRemoved waits for Android data directory to be removed.
func waitForAndroidDataRemoved(ctx context.Context) error {
	const androidDataDirPath = "/opt/google/containers/android/rootfs/android-data/data"

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(androidDataDirPath); !os.IsNotExist(err) {
			return errors.New("Android data still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for Android data directory to be removed")
	}
	return nil
}

// chromeOSVersion returns the Chrome OS version string. (e.g. "12345.0.0")
func chromeOSVersion() (string, error) {
	m, err := lsbrelease.Load()
	if err != nil {
		return "", err
	}
	val, ok := m[lsbrelease.Version]
	if !ok {
		return "", errors.Errorf("failed to find %s in /etc/lsb-release", lsbrelease.Version)
	}
	return val, nil
}

// dumpLogcat dumps logcat output to a log file in the test result directory.
func dumpLogcat(ctx context.Context, filePath string) error {
	logFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer logFile.Close()

	cmd := testexec.CommandContext(ctx, "android-sh", "-c", "logcat -d")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}
