// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

const (
	apk             = "ArcDEQPAndroidCTS-9.0.apk"
	testCaseDataDir = "deqp_tests"
	packageName     = "com.drawelements.deqp"
	activityName    = "android.app.NativeActivity"
)

// deqpTests contains the filenames of all the tests cases we want to run the dEQP on.
// These caselist files have been obtained by the official Android CTS suites:
// https://source.android.com/compatibility/cts/downloads.html
var deqpTests = []string{
	"egl-master.txt",
	"gles2-master.txt",
	"gles2-master-risky.txt",
	"gles3-master.txt",
	"gles3-master-risky.txt",
	"gles3-multisample.txt",
	"gles3-rotate-landscape.txt",
	"gles3-rotate-reverse-landscape.txt",
	"gles3-rotate-portrait.txt",
	"gles3-rotate-reverse-portrait.txt",
	"gles3-565-no-depth-no-stencil.txt",
	"gles31-master.txt",
	"gles31-master-risky.txt",
	"gles31-multisample.txt",
	"gles31-rotate-landscape.txt",
	"gles31-rotate-reverse-landscape.txt",
	"gles31-rotate-portrait.txt",
	"gles31-rotate-reverse-portrait.txt",
	"gles31-565-no-depth-no-stencil.txt",
}

func init() {
	var testCaseListFiles = []string{}
	for _, testCase := range deqpTests {
		testCaseListFiles = append(testCaseListFiles, testCaseDataDir+"/"+testCase)
	}
	testing.AddTest(&testing.Test{
		Func:         DEQP,
		Desc:         "Runs a subset of the DEQP test suite via the android CTS-provided apk binary",
		Contacts:     []string{"morg@chromium.org", "arc-eng@google.com"},
		Data:         append([]string{apk}, testCaseListFiles...),
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			Val:               1,
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_all_both"},
			Timeout:           240 * time.Minute, // Executes with a timeout of 4 hours accounting for enough time for the slower devices.
		}},
	})
}

// waitForFinish runs a polling operation on the state of a spawned activity and only returns when the activity ends, or the polling timeout expires.
func waitForFinish(ctx context.Context, a *arc.ARC, pollOptions *testing.PollOptions, pkgName string) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		tasks, err := a.DumpsysActivityActivities(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, task := range tasks {
			if task.PkgName == pkgName {
				return errors.New("match")
			}
		}
		return nil
	}, pollOptions)
	return err
}

// runSingleDEQPTest launches a single DEQP run on the given test caselist file and polls on it until it completes or times out.
func runSingleDEQPTest(ctx context.Context, a *arc.ARC, name string) error {
	fileName := "/sdcard/testcases/" + name
	testing.ContextLog(ctx, "Starting dEQP app for ", fileName)
	args := []string{"start"}
	args = append(args, "-W", "-S", "-n", packageName+"/"+activityName)
	args = append(args, "-e", "cmdLine", "deqp --deqp-log-filename=/sdcard/"+name+"-results.qpa --deqp-caselist-file="+fileName)
	cmd := a.Command(ctx, "am", args...)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed starting app for %v", name)
	}
	testing.ContextLog(ctx, "Waiting for activity to finish")
	testing.ContextLog(ctx, "Polling on dEQP state")
	err := waitForFinish(ctx, a, &testing.PollOptions{Interval: 500 * time.Millisecond}, packageName)
	return err
}

// pushCaseListFilesToAndroid pushes each individual caselist file to the Android device via adb push.
func pushCaseListFilesToAndroid(ctx context.Context, s *testing.State, a *arc.ARC) error {
	for _, testName := range deqpTests {
		if err := a.PushFile(ctx, s.DataPath(testCaseDataDir+"/"+testName), "/sdcard/testcases/"+testName); err != nil {
			return err
		}
	}
	return nil
}

func DEQP(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Installing dEQP APK")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// We need to push each file to the device because on ARCVM we do not expose the guest
	// filesystem to the host.
	s.Log("Pushing testcase files to Android device")
	if err := pushCaseListFilesToAndroid(ctx, s, a); err != nil {
		s.Fatal("Failed pushing testcase lists to Android: ", err)
	}

	failedTests := false
	for _, testName := range deqpTests {
		// Check if the testcase we want to run is empty. The dEQP APK does not like
		// empty caselists and breaks the tast run if we do not skip it.
		f, err := os.Open(s.DataPath(testCaseDataDir + "/" + testName))
		if err != nil {
			s.Fatalf("Failed to open %s: %v", testName, err)
		}
		defer f.Close()
		c, err := ioutil.ReadAll(f)
		if err != nil {
			s.Fatalf("Failed to read %s: %v", testName, err)
		}
		if len(strings.TrimSpace(string(c))) == 0 {
			s.Logf("Testcase %v is empty. Skipping", testName)
			continue
		}
		if err := runSingleDEQPTest(ctx, a, testName); err != nil {
			s.Fatalf("Test %s has failed: %v ", testName, err)
		}
		logFileOnGuest := "/sdcard/" + testName + "-results.qpa"
		logFileOnHost := s.OutDir() + testName + "-results.qpa"
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
		s.Logf("Tests run: %v. Tests failed: %v", testsRun, testsRun-len(nonFailed))
	}
	if failedTests {
		s.Fatal("Some dEQP tests did not pass")
	}
}
