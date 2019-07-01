// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package authperf

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// Number of passing ARC boots to collect results.
	successBootCount = 10

	// Number of maximum allowed boot errors.
	maxErrorBootCount = 1

	androidDataDirPath = "/opt/google/containers/android/rootfs/android-data/data"
	playStoreAppID     = "cnbgggchhmkkdmeppjobngjoejnihlei"
)

// Measured times in milliseconds.
type measuredValues struct {
	playStoreShownTime float64
	accountCheckTime   float64
	checkinTime        float64
	networkWaitTime    float64
	signInTime         float64
}

// RunTest passes multiple opt-ins and collects checkin, network wait sign-in
// and Play Store shown time. Average, min and max results are reported.
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

	for {
		if len(playStoreShownTimes) >= successBootCount {
			break
		}

		v, err := performArcBoot(ctx, s, cr, tconn)
		if err != nil {
			errorCount++
			logcatFileName := fmt.Sprintf("logcat_error_%d.log", errorCount)
			s.Logf("Error found during the ARC boot: %v - dumping logcat to %s",
				err, logcatFileName)

			dumpLogcat(ctx, s, logcatFileName)
			if err != nil {
				s.Log("Can't dump logcat: ", err)
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
	defer perfValues.Save(s.OutDir())

	// Comma separated results that can be easily pasted into Google sheets.
	var resultForLog []string

	// TODO(niwa): Get ChromeOS release version (e.g. 12307.0.0) and append.
	// resultForLog = append(resultForLog, utils.get_chromeos_release_version())
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

	reportResult("plsy_store_shown_time", playStoreShownTimes)
	reportResult("account_check_time", accountCheckTimes)
	reportResult("checkin_time", checkinTimes)
	reportResult("network_wait_time", networkWaitTimes)
	reportResult("sign_in_time", signInTimes)

	s.Log(strings.Join(resultForLog, ","))
}

// Performs one ARC boot iteration, opt-out and opt-in again.
// Calculate the time when the Play Store appears and set of ARC auth times.
func performArcBoot(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.Conn) (measuredValues, error) {
	v := measuredValues{}

	// Opt-out and wait.
	err := setPlayStoreEnabled(ctx, s, tconn, false)
	if err != nil {
		return v, err
	}
	err = waitForAndroidDataRemoved(ctx, s)
	if err != nil {
		return v, err
	}

	startTime := time.Now()

	// Opt-in and wait.
	err = optInArc(ctx, s, cr, tconn)
	if err != nil {
		return v, err
	}
	err = waitForPlayStoreShown(ctx, s, tconn)
	if err != nil {
		return v, err
	}

	v.playStoreShownTime = time.Now().Sub(startTime).Seconds() * 1000

	// Extracts specified time value from a logcat entry formatted like this:
	// "I/ArcProvisioning: SignIn result: Sign-in succeeded in ???? ms.
	//  Account check completed ???? ms with status 1. Network waited ???? ms.
	//  Checkin waited ???? ms in 1 attempts"
	timeFromLogEntry := func(logEntry string, prefix string) float64 {
		startPos := strings.Index(logEntry, prefix)
		if startPos < 0 {
			s.Fatalf("Could not extract time %s from %s", prefix, logEntry)
		}
		substr := logEntry[startPos+len(prefix):]
		endPos := strings.Index(substr, " ms")
		val, err := strconv.ParseFloat(substr[:endPos], 64)
		if err != nil {
			s.Fatalf("Could not extract time %s from %s", prefix, logEntry)
		}
		return val
	}

	// Finds the target entry in logcat and parses it with timeFromLogEntry.
	cmd := "logcat -v tag -d ArcProvisioning:I *:S"
	out, err := testexec.CommandContext(ctx, "android-sh", "-c", cmd).Output()
	if err != nil {
		return v, errors.Wrapf(err, "failed to run %v", cmd)
	}

	var timesDetected = false
	for _, l := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(l, "I/ArcProvisioning: SignIn result: ") {
			continue
		}
		timesDetected = true
		v.signInTime = timeFromLogEntry(l, "Sign-in succeeded in ")
		v.accountCheckTime = timeFromLogEntry(l, "Account check completed ")
		v.networkWaitTime = timeFromLogEntry(l, "Network waited ")
		v.checkinTime = timeFromLogEntry(l, "Checkin waited ")
		break
	}

	if !timesDetected {
		return v, errors.New("Sign-in results were not found")
	}

	return v, nil
}

// Wrapper for chrome.autotestPrivate.setPlayStoreEnabled.
// Ported from arc_util.enable_play_store
func setPlayStoreEnabled(ctx context.Context, s *testing.State, tconn *chrome.Conn, enabled bool) error {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.autotestPrivate.setPlayStoreEnabled(%s, () => {
				if (chrome.runtime.lastError === undefined) {
					resolve();
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`, strconv.FormatBool(enabled))
	return tconn.EvalPromise(ctx, expr, nil)
}

// Step through opt in and wait for it to complete.
// Ported from arc_util.opt_in.
func optInArc(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.Conn) error {
	s.Log("Initializing ARC opt-in flow")

	setPlayStoreEnabled(ctx, s, tconn, true)

	bgURL := chrome.ExtensionBackgroundPageURL(playStoreAppID)
	f := func(t *chrome.Target) bool { return t.URL == bgURL }
	conn, err := cr.NewConnForTarget(ctx, f)
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
			return errors.Wrapf(err, "Error waiting for %v", condition)
		}
	}

	expr := "termsPage.onAgree()"
	if err = conn.Exec(ctx, expr); err != nil {
		return errors.Wrapf(err, "Error executing %v", expr)
	}

	cond := "!appWindow"
	if err := conn.WaitForExpr(ctx, cond); err != nil {
		return errors.Wrapf(err, "Error waiting for %v", cond)
	}

	// TODO(niwa): Check if we still need to handle non-tos_needed case.

	s.Log("ARC opt-in flow complete")
	return nil
}

func waitForAndroidDataRemoved(ctx context.Context, s *testing.State) error {
	s.Log("Waiting for Android data directory to be removed")

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

func waitForPlayStoreShown(ctx context.Context, s *testing.State, tconn *chrome.Conn) error {
	s.Log("Waiting for Play Store window to be shown")

	playStoreWindowShown := func() bool {
		var appShown bool
		expr := fmt.Sprintf(
			`new Promise((resolve, reject) => {
				chrome.autotestPrivate.isAppShown('%v', function(appShown) {
					if (chrome.runtime.lastError === undefined) {
						resolve(appShown);
					} else {
						reject(chrome.runtime.lastError.message);
					}
				});
			})`, playStoreAppID)
		if err := tconn.EvalPromise(ctx, expr, &appShown); err != nil {
			s.Error("Running autotestPrivate.isAppShown failed: ", err)
			return false
		}
		return appShown
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if playStoreWindowShown() {
			return nil
		}
		return errors.New("Play Store is not shown yet")
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return errors.Wrap(err,
			"failed to wait for Play Store window to be shown")
	}
	return nil
}

// Dumps logcat output to a log file in the test result directory.
func dumpLogcat(ctx context.Context, s *testing.State, fileName string) error {
	logFile, err := os.Create(filepath.Join(s.OutDir(), fileName))
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer logFile.Close()

	cmd := testexec.CommandContext(ctx, "android-sh", "-c", "logcat -d")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}
