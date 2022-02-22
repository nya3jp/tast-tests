// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/camera/pre"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type itsParam struct {
	// Scene is the ITS test scene number.
	Scene int
	// Facing is the test camera facing.
	Facing pb.Facing
	// ChartPath specify the chart path in ITS zip file. Maybe empty for no need to display chart.
	ChartPath string
}

type rule struct {
	pat     *regexp.Regexp
	handler func([]string) error
}

// match checks if the |line| matches this rule and runs corresponding handler
// when matched. Returns boolean for whether the rule is matched.
func (r *rule) match(line string) (bool, error) {
	m := r.pat.FindStringSubmatch(line)
	if len(m) == 0 {
		return false, nil
	}
	if err := r.handler(m[1:]); err != nil {
		return false, errors.Wrapf(err, "failed to execute rule handler with pattern %v", r.pat)
	}
	return true, nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ITS,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies camera HAL3 interface function on remote DUT",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		Data:         []string{"adb", pre.SetupITSRepoScript, pre.ITSPy3Patch},
		Vars:         []string{"chart"},
		SoftwareDeps: []string{"chrome", "android_p", "arc_camera3", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.ITSService"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{
			// X86
			{
				Name:              "scene0_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         append([]string{pre.CtsVerifierX86Zip}),
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{0, pb.Facing_FACING_BACK, ""},
			},
		},
	})
}

// sceneResult is the summary of test result under same test scene.
type sceneResult struct {
	// name is the test scene name.
	name string
	// total is the non-skipped test item count.
	total int
	// pass is the passed test item count.
	pass int
}

func ITS(ctx context.Context, s *testing.State) {
	// Reserve extra time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	its := (s.PreValue().(*pre.ITSHelper))
	param := s.Param().(itsParam)
	camID, err := its.CameraID(ctx, param.Facing)
	if err != nil {
		s.Fatalf("Failed to get camera id of camera facing %s: %v", param.Facing, err)
	}

	// Prepare test scene chart.
	if len(param.ChartPath) > 0 {
		testing.ContextLogf(ctx, "Displaying chart for scene%d", param.Scene)

		var altHostname string
		if hostname, ok := s.Var("chart"); ok {
			altHostname = hostname
		}
		c, namePaths, err := chart.New(ctx, s.DUT(), altHostname, s.OutDir(), []string{param.ChartPath})
		if err != nil {
			s.Fatal("Failed to prepare chart tablet: ", err)
		}
		defer func(ctx context.Context) {
			if err := c.Close(ctx, s.OutDir()); err != nil {
				s.Error("Failed to cleanup chart: ", err)
			}
		}(cleanupCtx)

		if err := c.Display(ctx, namePaths[0]); err != nil {
			s.Fatal("Failed to display chart on chart tablet: ", err)
		}
	}

	testing.ContextLog(ctx, "Running ITS")
	cmd := its.TestCmd(ctx, param.Scene, camID)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to get stdout pipe of ITS command: ", err)
	}
	defer stdout.Close()
	stdoutLog, err := os.Create(path.Join(s.OutDir(), "its_stdout.log"))
	if err != nil {
		s.Fatal("Failed to create ITS stdout log file: ", err)
	}
	defer stdoutLog.Close()

	stderrLog, err := os.Create(path.Join(s.OutDir(), "its_stderr.log"))
	if err != nil {
		s.Fatal("Failed to create ITS stderr log file: ", err)
	}
	defer stderrLog.Close()
	cmd.Stderr = stderrLog

	sceneResults := make(chan *sceneResult, 1)
	itsLogPath := ""
	// Goroutine for pulling stdout, the main thread waits for cmd exiting.
	go func() {
		defer close(sceneResults)

		result := sceneResult{"", -1, -1}
		// Rules for parsing lines from stdout/stderr of ITS test with |pat| and run the corresponding |handler|.
		// Patterns are reference from https://cs.android.com/android/platform/superproject/+/android-9.0.0_r16:cts/apps/CameraITS/tools/run_all_tests.py
		sequentialRules := []rule{
			{regexp.MustCompile(`Saving output files to: (\S+)`), func(m []string) error {
				itsLogPath = m[0]
				testing.ContextLog(ctx, "ITS output log: ", itsLogPath)
				return nil
			}},
			{regexp.MustCompile(`Start running ITS on camera \d+, (scene\d+)`), func(m []string) error {
				result.name = m[0]
				testing.ContextLog(ctx, "Starting test scene: ", result.name)
				return nil
			}},
			{regexp.MustCompile(`(\d+) / (\d+) tests passed`), func(m []string) error {
				pass, err := strconv.Atoi(m[0])
				if err != nil {
					return errors.Wrapf(err, "failed to convert parsed test passed count %v to number ", m[0])
				}
				total, err := strconv.Atoi(m[1])
				if err != nil {
					return errors.Wrapf(err, "failed to convert parsed test total count %v to number ", m[1])
				}
				testing.ContextLogf(ctx, "Test result %d passed in total %d tests", pass, total)
				result.pass = pass
				result.total = total
				return nil
			}},
		}
		repeatedRule := rule{regexp.MustCompile(`(\S+\s+scene\d+/\S+\s+\[[\d.]+s\])`), func(m []string) error {
			// Example: SKIP  scene0/test_unified_timestamps [1.9s]
			testing.ContextLog(ctx, m[0])
			return nil
		}}

		ruleIndex := 0
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutLog.Write([]byte(line + "\n"))

			if ruleIndex < len(sequentialRules) {
				r := sequentialRules[ruleIndex]
				m, err := r.match(line)
				if err != nil {
					s.Fatal("Failed to match rule: ", err)
				}
				if m {
					ruleIndex++
				}
			}
			if _, err := repeatedRule.match(line); err != nil {
				s.Fatal("Failed to match rule: ", err)
			}
		}
		if err := scanner.Err(); err != nil {
			s.Error("Encountered error when scanning output: ", err)
		}
		if result.name == "" {
			s.Fatal("No result name presented")
		}
		if result.total == -1 {
			s.Fatal("No result total number presented")
		}
		if result.total == -1 {
			s.Fatal("No result pass number presented")
		}
		sceneResults <- &result
	}()

	// TODO(b/169303835): Collect /var/log/messages during test run.
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start ITS: ", err)
	}
	defer func(ctx context.Context) {
		if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to run ITS: ", err)
		}
	}(ctx)
	defer func(ctx context.Context) {
		if itsLogPath == "" {
			s.Error("No ITS log path")
			return
		}
		p := path.Join(s.OutDir(), "its")
		if err := os.Rename(itsLogPath, p); err != nil {
			s.Errorf("Failed to move ITS log path %v to test output directory: %v", itsLogPath, err)
		}
	}(cleanupCtx)

	r, ok := <-sceneResults
	if ok {
		if r.total != r.pass {
			s.Errorf("Failed in test scene %v with pass rate %d/%d", r.name, r.pass, r.total)
		}
	} else {
		s.Error("No test result from ITS")
	}
}
