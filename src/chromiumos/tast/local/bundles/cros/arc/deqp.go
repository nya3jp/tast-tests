// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

// deqpTestsFull and deqpTestsMinimal contain the filenames of the tests cases
// we want to run the dEQP on. Minimal is to be used on short tast runs that
// only take a few minutes to execute; it is always a subset of the full list.
// These caselist files have been obtained by the official Android CTS suites:
// https://source.android.com/compatibility/cts/downloads.html
var deqpTestsFull = []string{
	"egl-master.txt",
	"gles2-master.txt",
	"gles3-master.txt",
	"gles3-multisample.txt",
	"gles3-rotate-landscape.txt",
	"gles3-rotate-reverse-landscape.txt",
	"gles3-rotate-portrait.txt",
	"gles3-rotate-reverse-portrait.txt",
	"gles3-565-no-depth-no-stencil.txt",
	"gles31-master.txt",
	"gles31-multisample.txt",
	"gles31-rotate-landscape.txt",
	"gles31-rotate-reverse-landscape.txt",
	"gles31-rotate-portrait.txt",
	"gles31-rotate-reverse-portrait.txt",
	"gles31-565-no-depth-no-stencil.txt",
}

var deqpTestsMinimal = []string{
	"gles3-multisample.txt",
	"gles31-multisample.txt",
}

func init() {
	var testCaseListFiles = []string{}
	for _, testCase := range deqpTestsFull {
		testCaseListFiles = append(testCaseListFiles, testCaseDataDir+"/"+testCase)
	}
	testing.AddTest(&testing.Test{
		Func:         DEQP,
		Desc:         "Runs a subset of the DEQP test suite via the android CTS-provided apk binary",
		Contacts:     []string{"morg@chromium.org", "arc-eng@google.com"},
		Data:         append([]string{apk}, testCaseListFiles...),
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
			Val:               "minimal",
			Timeout:           5 * time.Minute,
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:mainline", "informational"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
			Val:               "minimal",
			Timeout:           5 * time.Minute,
		}, {
			Name:              "full",
			ExtraAttr:         []string{"group:graphics", "graphics_nightly"},
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
			Timeout:           4 * time.Hour,
			Val:               "full",
		}, {
			Name:              "full_vm",
			ExtraAttr:         []string{"group:graphics", "graphics_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
			Timeout:           4 * time.Hour,
			Val:               "full",
		}},
	})
}

func runSingleDEQPTest(ctx context.Context, a *arc.ARC, name string) error {
	fileName := "/sdcard/testcases/" + name
	testing.ContextLog(ctx, "Starting dEQP app for ", fileName)

	act, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create new activity for %v", name)
	}
	defer act.Close()
	prefixArgs := []string{"-W", "-S", "-n"}
	suffixArgs := []string{"-e", "cmdLine", "deqp --deqp-log-filename=/sdcard/" + name + "-results.qpa --deqp-caselist-file=" + fileName}
	if err := act.StartWithArgs(ctx, prefixArgs, suffixArgs); err != nil {
		return errors.Wrapf(err, "failed to start activity for %v", name)
	}

	return act.WaitForFinished(ctx, 0*time.Second)
}

// pushCaseListFilesToAndroid pushes each individual caselist file to the Android device via adb push.
func pushCaseListFilesToAndroid(ctx context.Context, s *testing.State, a *arc.ARC, deqpTests []string) error {
	for _, testName := range deqpTests {
		if err := a.PushFile(ctx, s.DataPath(testCaseDataDir+"/"+testName), "/sdcard/testcases/"+testName); err != nil {
			return err
		}
	}
	return nil
}

func DEQP(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	var deqpTests = []string{}
	if s.Param().(string) == "full" {
		deqpTests = deqpTestsFull
	} else {
		deqpTests = deqpTestsMinimal
	}

	s.Log("Installing dEQP APK")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// We need to push each file to the device because on ARCVM we do not expose the guest
	// filesystem to the host.
	s.Log("Pushing testcase files to Android device")
	if err := pushCaseListFilesToAndroid(ctx, s, a, deqpTests); err != nil {
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
