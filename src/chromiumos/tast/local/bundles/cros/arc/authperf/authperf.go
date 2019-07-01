// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package authperf implements the tests for measuring the performance of
// ARC opt-in flow and auth time.
package authperf

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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

const (
	// successBootCount is the number of passing ARC boots to collect results.
	successBootCount = 10

	// maxErrorBootCount is the number of maximum allowed boot errors.
	maxErrorBootCount = 1

	androidDataDirPath = "/opt/google/containers/android/rootfs/android-data/data"
	playStoreAppID     = "cnbgggchhmkkdmeppjobngjoejnihlei"
)

// measuredValues stores measured times in milliseconds.
type measuredValues struct {
	playStoreShownTime float64
	accountCheckTime   float64
	checkinTime        float64
	networkWaitTime    float64
	signInTime         float64
}

// RunTest steps through multiple opt-ins and collects checkin time, network
// wait time, sign-in time and Play Store shown time.
// It also reports average, min and max results.
func RunTest(ctx context.Context, s *testing.State, username string, password string, resultSuffix string) {
	cr, err := chrome.New(ctx, chrome.ARCEnabledWithPlayStore(),
		chrome.GAIALogin(), chrome.Auth(username, password, ""),
		chrome.ExtraArgs("--arc-force-show-optin-ui"))
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

	for len(playStoreShownTimes) < successBootCount {
		s.Logf("Running ARC opt-in iteration #%d out of %d",
			len(playStoreShownTimes)+1, successBootCount)

		v, err := bootARC(ctx, s, cr, tconn)
		if err != nil {
			errorCount++
			logcatFileName := fmt.Sprintf("logcat_error_%d.log", errorCount)
			s.Logf("Error found during the ARC boot: %v - dumping logcat to %s",
				err, logcatFileName)

			if err := dumpLogcat(ctx, s.OutDir(), logcatFileName); err != nil {
				s.Log("Failed to dump logcat: ", err)
			}

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
	}

	perfValues := perf.NewValues()

	// Array for generating comma separated results.
	var resultForLog []string

	version, err := chromeOSVersion()
	if err != nil {
		s.Fatal("Can't get ChromeOS version: ", err)
	}
	resultForLog = append(resultForLog, version)
	resultForLog = append(resultForLog, strconv.Itoa(len(playStoreShownTimes)))

	reportResult := func(name string, samples []float64) {
		name = name + resultSuffix

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

	// Outputs comma separated results to the log.
	// It is used in many sheet tables to automate paste operations.
	s.Log(strings.Join(resultForLog, ","))

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// bootARC performs one ARC boot iteration, opt-out and opt-in again.
// It calculates the time when the Play Store appears and set of ARC auth times.
func bootARC(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.Conn) (measuredValues, error) {
	v := measuredValues{}

	// Opt out.
	if err := setPlayStoreEnabled(ctx, tconn, false); err != nil {
		return v, err
	}

	s.Log("Waiting for Android data directory to be removed")
	if err := waitForAndroidDataRemoved(ctx); err != nil {
		return v, err
	}

	startTime := time.Now()

	// Opt in
	s.Log("Waiting for ARC opt-in flow to complete")
	if err := optInARC(ctx, cr, tconn); err != nil {
		return v, err
	}

	s.Log("Waiting for Play Store window to be shown")
	if err := waitForPlayStoreShown(ctx, tconn); err != nil {
		return v, err
	}

	v.playStoreShownTime = time.Now().Sub(startTime).Seconds() * 1000

	out, err := testexec.CommandContext(ctx, "android-sh", "-c",
		"logcat -v tag -d ArcProvisioning:I *:S").Output(testexec.DumpLogOnError)
	if err != nil {
		return v, errors.Wrap(err, "failed to run logcat")
	}

	extractTime := func(logEntry string, re *regexp.Regexp) (float64, error) {
		m := re.FindStringSubmatch(logEntry)
		if m == nil {
			return 0, errors.Errorf("failed to match %v on %s", re, logEntry)
		}
		val, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, err
		}
		return val, nil
	}

	// Extracts time values from the "SignIn result" logcat entry.
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "I/ArcProvisioning: SignIn result: ") {
			v.signInTime, err = extractTime(
				l, regexp.MustCompile(`Sign-in succeeded in (\d+) ms`))
			if err != nil {
				return v, err
			}
			v.accountCheckTime, err = extractTime(
				l, regexp.MustCompile(`Account check completed (\d+) ms`))
			if err != nil {
				return v, err
			}
			v.networkWaitTime, err = extractTime(
				l, regexp.MustCompile(`Network waited (\d+) ms`))
			if err != nil {
				return v, err
			}
			v.checkinTime, err = extractTime(
				l, regexp.MustCompile(`Checkin waited (\d+) ms`))
			if err != nil {
				return v, err
			}
			return v, nil
		}
	}

	return v, errors.New("Sign-in results were not found")
}

// setPlayStoreEnabled is a wrapper for chrome.autotestPrivate.setPlayStoreEnabled.
func setPlayStoreEnabled(ctx context.Context, tconn *chrome.Conn, enabled bool) error {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.setPlayStoreEnabled(%t, () => {
			 if (chrome.runtime.lastError) {
			   reject(new Error(chrome.runtime.lastError.message));
			   return;
			 }
			 resolve();
		   });
		 })`, enabled)
	return tconn.EvalPromise(ctx, expr, nil)
}

// optInARC steps through opt-in flow and wait for it to complete.
func optInARC(ctx context.Context, cr *chrome.Chrome, tconn *chrome.Conn) error {
	setPlayStoreEnabled(ctx, tconn, true)

	bgURL := chrome.ExtensionBackgroundPageURL(playStoreAppID)
	conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
		return t.URL == bgURL
	})
	if err != nil {
		return errors.Wrapf(err, "failed to find %v", bgURL)
	}
	defer conn.Close()

	var waitConditions = []string{
		"(port != null)",
		"(termsPage != null)",
		"(termsPage.isManaged_ || termsPage.state_ == LoadState.LOADED)",
	}
	for _, condition := range waitConditions {
		if err := conn.WaitForExpr(ctx, condition); err != nil {
			return errors.Wrapf(err, "failed to wait for %v", condition)
		}
	}

	if err := conn.Exec(ctx, "termsPage.onAgree()"); err != nil {
		return errors.Wrap(err, "failed to execute 'termsPage.onAgree()'")
	}

	if err := conn.WaitForExpr(ctx, "!appWindow"); err != nil {
		return errors.Wrap(err, "failed to wait for '!appWindow'")
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.
	return nil
}

// waitForAndroidDataRemoved waits for Android data directory to be removed.
func waitForAndroidDataRemoved(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(androidDataDirPath); os.IsExist(err) {
			return errors.New("Android data still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err,
			"failed to wait for Android data directory to be removed")
	}
	return nil
}

// waitForPlayStoreShown waits for Play Store window to be shown.
func waitForPlayStoreShown(ctx context.Context, tconn *chrome.Conn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var appShown bool
		expr := fmt.Sprintf(
			`new Promise((resolve, reject) => {
			   chrome.autotestPrivate.isAppShown('%s', function(appShown) {
				 if (chrome.runtime.lastError) {
				   reject(chrome.runtime.lastError.message);
				   return;
				 }
				 resolve(appShown);
			   });
			 })`, playStoreAppID)
		if err := tconn.EvalPromise(ctx, expr, &appShown); err != nil {
			return testing.PollBreak(err)
		}
		if !appShown {
			return errors.New("Play Store is not shown yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return errors.Wrap(err,
			"failed to wait for Play Store window to be shown")
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
func dumpLogcat(ctx context.Context, outDir string, fileName string) error {
	logFile, err := os.Create(filepath.Join(outDir, fileName))
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer logFile.Close()

	cmd := testexec.CommandContext(ctx, "android-sh", "-c", "logcat -d")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}
