// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	apkCTS9         = "ArcDEQPAndroidCTS-9.0r9_20191126.apk"
	apkCTS11        = "ArcDEQPAndroidCTS-11.0r1_20200924.apk"
	testCaseDataDir = "deqp_tests"
	packageName     = "com.drawelements.deqp"
	activityName    = "android.app.NativeActivity"
)

// deqpTests contain the filenames of the tests cases we want to run the
// dEQP on. These caselist files have been obtained by the official Android
// CTS suites: https://source.android.com/compatibility/cts/downloads.html
// TODO(morg): Set up a DEQP "full" test suite for nightly builds.
var deqpTests = []string{
	filepath.Join(testCaseDataDir, "gles3-multisample.txt"),
	filepath.Join(testCaseDataDir, "gles31-multisample.txt"),
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DEQP,
		Desc:         "Runs a subset of the DEQP test suite via the android CTS-provided apk binary",
		Contacts:     []string{"morg@chromium.org", "arc-eng@google.com"},
		Data:         deqpTests,
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraData:         []string{apkCTS9},
			Val:               apkCTS9,
			Timeout:           5 * time.Minute,
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData:         []string{apkCTS11},
			Val:               apkCTS11,
			Timeout:           5 * time.Minute,
		}},
	})
}

func runSingleDEQPTest(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, name string) error {
	fileName := filepath.Join("/sdcard/testcases", name)
	testing.ContextLog(ctx, "Starting dEQP app for ", fileName)

	act, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create new activity for %v", name)
	}
	defer act.Close()

	prefixArgs := []string{"-W", "-S", "-n"}
	suffixArgs := []string{"-e", "cmdLine", "deqp --deqp-log-filename=/sdcard/" + name + "-results.qpa --deqp-caselist-file=" + fileName}
	if err := act.StartWithArgs(ctx, tconn, prefixArgs, suffixArgs); err != nil {
		return errors.Wrapf(err, "failed to start activity for %v", name)
	}

	return act.WaitForFinished(ctx, ctxutil.MaxTimeout)
}

// pushCaseListFilesToAndroid pushes each individual caselist file to the Android device via adb push.
func pushCaseListFilesToAndroid(ctx context.Context, s *testing.State, a *arc.ARC) error {
	for _, testFile := range deqpTests {
		testName := filepath.Base(testFile)
		if err := a.PushFile(ctx, s.DataPath(testFile), filepath.Join("/sdcard/testcases", testName)); err != nil {
			return err
		}
	}
	return nil
}

// DEQP is the main tast test entry point.
// TODO(morg): Add support for different DEQP suite versions as a parameterized test flag (9.0r9, 9.0r10, etc).
func DEQP(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	apk := s.Param().(string)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing dEQP APK")
	if err := a.Install(ctx, s.DataPath(apk), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// Grant apk permissions to manage external storage. This is required
	// for Android 11 devices.
	if apk == apkCTS11 {
		cmd := a.Command(ctx, "appops", "set", packageName, "MANAGE_EXTERNAL_STORAGE", "allow")
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to set app permissions: ", err)
		}
	}

	// We need to push each file to the device because on ARCVM we do not expose the guest
	// filesystem to the host.
	s.Log("Pushing testcase files to Android device")
	if err := pushCaseListFilesToAndroid(ctx, s, a); err != nil {
		s.Fatal("Failed pushing testcase lists to Android: ", err)
	}

	failedTests := false
	totalTests := 0
	totalFailedTests := 0
	for _, testFile := range deqpTests {
		testName := filepath.Base(testFile)
		c, err := ioutil.ReadFile(s.DataPath(testFile))
		if err != nil {
			s.Fatalf("Failed to read %s: %v", testFile, err)
		}
		if len(strings.TrimSpace(string(c))) == 0 {
			s.Logf("Testcase %v is empty. Skipping", testFile)
			continue
		}

		if err := runSingleDEQPTest(ctx, a, tconn, testName); err != nil {
			s.Fatalf("Test %s has failed: %v ", testName, err)
		}
		logFileOnGuest := filepath.Join("/sdcard", testName+"-results.qpa")
		logFileOnHost := filepath.Join(s.OutDir(), testName+"-results.qpa")
		if err := a.PullFile(ctx, logFileOnGuest, logFileOnHost); err != nil {
			s.Fatalf("Failed to obtain test results for %s: %v", testName, err)
		}
		stats, nonFailed, err := graphics.ParseDEQPOutput(logFileOnHost)
		if err != nil {
			s.Logf("Failed to parse results for %s: %v", logFileOnHost, err)
			s.Logf("The test for %s will be considered failed", testName)
			failedTests = true
		}
		testsRun := 0
		s.Logf("Parsing %v results", testName)
		for r, c := range stats {
			if c > 0 && graphics.DEQPOutcomeIsFailure(r) {
				failedTests = true
			}
			testsRun += int(c)
		}
		s.Logf("%v: %v", testName, stats)
		s.Logf("Tests run: %v. Tests considered failed: %v", testsRun, testsRun-len(nonFailed))
		totalTests += testsRun
		totalFailedTests += testsRun - len(nonFailed)
	}
	if failedTests {
		s.Fatalf("Some dEQP tests did not pass: %v(failed)/%v(total)", totalFailedTests, totalTests)
	}
}
