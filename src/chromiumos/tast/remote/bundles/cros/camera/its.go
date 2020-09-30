// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bufio"
	"context"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/remote/bundles/cros/camera/chart"
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
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ITS,
		Desc:         "Verifies camera HAL3 interface function on remote DUT",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		Data:         []string{"adb"},
		Vars:         []string{"chart"},
		SoftwareDeps: []string{"chrome", "android_p", "arc_camera3", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.ITSService"},
		Timeout:      15 * time.Minute,
		Params: []testing.Param{
			// X86
			testing.Param{
				Name:              "scene0_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         append([]string{pre.CTSVerifierX86.DataPath()}, pre.CTSVerifierX86.PatchesDataPathes()...),
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{0, pb.Facing_FACING_BACK},
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
	chartPath := its.Chart(param.Scene)
	if _, err := os.Stat(chartPath); err != nil {
		if os.IsNotExist(err) {
			testing.ContextLogf(ctx, "No chart %v, skip display chart for scene%d", chartPath, param.Scene)
		} else {
			s.Fatalf("Failed to check chart file %v: %v", chartPath, err)
		}
	} else {
		testing.ContextLogf(ctx, "Display chart for scene%d", param.Scene)

		var altHostname string
		if hostname, ok := s.Var("chart"); ok {
			altHostname = hostname
		}
		c, err := chart.New(ctx, s.DUT(), altHostname, chartPath, s.OutDir())
		if err != nil {
			s.Fatal("Failed to prepare chart tablet: ", err)
		}
		defer func(ctx context.Context) {
			if err := c.Close(ctx, s.OutDir()); err != nil {
				s.Error("Failed to cleanup chart: ", err)
			}
		}(cleanupCtx)
	}

	testing.ContextLog(ctx, "Run ITS")
	cmd := its.TestCmd(ctx, param.Scene, camID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to get stdout pipe of ITS command: ", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.Fatal("Failed to get stderr pipe of ITS command: ", err)
	}
	sceneResults := make(chan *sceneResult, 1)
	itsLogPath := ""
	go func() {
		defer stdout.Close()
		defer stderr.Close()
		defer close(sceneResults)
		result := sceneResult{"", -1, -1}
		// Rules for parsing lines from stdout/stderr of ITS test with |pat| and run the corresponding |handler|.
		// Patterns are reference from https://cs.android.com/android/platform/superproject/+/android-9.0.0_r16:cts/apps/CameraITS/tools/run_all_tests.py
		var rules = []struct {
			pat     *regexp.Regexp
			handler func([]string) error
		}{
			{regexp.MustCompile(`Saving output files to: (\S+)`), func(m []string) error {
				itsLogPath = m[0]
				testing.ContextLog(ctx, "ITS output log: ", itsLogPath)
				return nil
			}},
			{regexp.MustCompile(`Start running ITS on camera \d+, (scene\d+)`), func(m []string) error {
				result.name = m[0]
				testing.ContextLog(ctx, "Start test scene: ", result.name)
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

		ruleIndex := 0
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			testing.ContextLog(ctx, "[ITS] ", line)
			if ruleIndex >= len(rules) {
				continue
			}
			r := rules[ruleIndex]
			m := r.pat.FindStringSubmatch(line)
			if len(m) == 0 {
				continue
			}
			if err := r.handler(m[1:]); err != nil {
				s.Fatal("Failed to parse ITS stdout, stderr: ", err)
			}
			ruleIndex++
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

	if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run ITS: ", err)
	}
}
