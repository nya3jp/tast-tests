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
		Contacts:     []string{"morg@chromium.org", "arc-core@google.com"},
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
func waitForFinish(ctx context.Context, s *testing.State, pollOptions *testing.PollOptions, pkgName string) error {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Waiting for activity to finish")
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
func runSingleDEQPTest(ctx context.Context, s *testing.State, name string) error {
	a := s.PreValue().(arc.PreData).ARC
	fileName := "/sdcard/testcases/" + name
	s.Log("Starting dEQP app for ", fileName)
	cmd := a.Command(ctx, "am", "start", "-W", "-n", packageName+"/"+activityName)
	// For some reason, we need to append the extra args in a separate line instead of formatting them in the original Command() line, otherwise
	// the dEQP activity will not pick them up and won't work.
	cmd.Cmd.Args = append(cmd.Cmd.Args, "-e", "cmdLine \"deqp --deqp-log-filename=/sdcard/"+name+"-results.qpa --deqp-caselist-file="+fileName+"\"")
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed starting app for ", name, ": ", err)
	}
	s.Log("Polling on dEQP state")
	err := waitForFinish(ctx, s, &testing.PollOptions{Interval: 500 * time.Millisecond}, packageName)
	return err
}

// pushCaseListFilesToAndroid pushes each individual caselist file to the Android device via adb push.
func pushCaseListFilesToAndroid(ctx context.Context, s *testing.State) error {
	a := s.PreValue().(arc.PreData).ARC
	for _, testName := range deqpTests {
		s.Logf("Pushing %s to device", testName)
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
	if err := pushCaseListFilesToAndroid(ctx, s); err != nil {
		s.Fatal("Failed pushing testcase lists to Android: ", err)
	}

	failedTests := false
	for _, testName := range deqpTests {
		// Check if the testcase we want to run is empty. The dEQP APK does not like
		// empty caselists and breaks the tast run if we do not skip it.
		testFile, err := os.Open(s.DataPath(testCaseDataDir + "/" + testName))
		if err != nil {
			s.Fatal("Could not open ", testName)
		}
		contents, err := ioutil.ReadAll(testFile)
		if err != nil {
			s.Fatal("Could not read ", testName)
		}
		if len(strings.TrimSpace(string(contents))) == 0 {
			s.Logf("Testcase %v is empty. Skipping", testName)
			continue
		}
		if err := runSingleDEQPTest(ctx, s, testName); err != nil {
			s.Fatal("Test ", testName, " has failed: ", err)
		}
		logFileOnGuest := "/sdcard/" + testName + "-results.qpa"
		logFileOnHost := s.OutDir() + testName + "-results.qpa"
		if err := a.PullFile(ctx, logFileOnGuest, logFileOnHost); err != nil {
			s.Fatal("Failed to obtain test results for ", testName, ": ", err)
		}
		stats, nonFailed, err := graphics.ParseDEQPOutput(logFileOnHost)
		if err != nil {
			s.Log("Failed to parse results for ", logFileOnHost)
			s.Log("The test for ", testName, " will be considered failed.")
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
		s.Error("Some dEQP tests did not pass")
	}
}
