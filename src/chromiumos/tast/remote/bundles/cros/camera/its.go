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
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			// X86
			testing.Param{
				Name:              "scene0_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{0, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:              "scene0_front_x86",
				ExtraAttr:         []string{"camerabox_facing_front"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{0, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:              "scene1_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{1, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:              "scene1_front_x86",
				ExtraAttr:         []string{"camerabox_facing_front"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{1, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:              "scene2_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{2, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:              "scene2_front_x86",
				ExtraAttr:         []string{"camerabox_facing_front"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{2, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:              "scene3_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{3, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:              "scene3_front_x86",
				ExtraAttr:         []string{"camerabox_facing_front"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{3, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:              "scene4_back_x86",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{4, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:              "scene4_front_x86",
				ExtraAttr:         []string{"camerabox_facing_front"},
				ExtraData:         []string{pre.CTSVerifierX86.S()},
				ExtraHardwareDeps: hwdep.D(hwdep.X86()),
				Pre:               pre.ITSX86Pre,
				Val:               itsParam{4, pb.Facing_FACING_FRONT},
			},

			// ARM
			testing.Param{
				Name:      "scene0_back_arm",
				ExtraAttr: []string{"camerabox_facing_back"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{0, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "scene0_front_arm",
				ExtraAttr: []string{"camerabox_facing_front"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{0, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:      "scene1_back_arm",
				ExtraAttr: []string{"camerabox_facing_back"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{1, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "scene1_front_arm",
				ExtraAttr: []string{"camerabox_facing_front"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{1, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:      "scene2_back_arm",
				ExtraAttr: []string{"camerabox_facing_back"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{2, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "scene2_front_arm",
				ExtraAttr: []string{"camerabox_facing_front"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{2, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:      "scene3_back_arm",
				ExtraAttr: []string{"camerabox_facing_back"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{3, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "scene3_front_arm",
				ExtraAttr: []string{"camerabox_facing_front"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{3, pb.Facing_FACING_FRONT},
			},
			testing.Param{
				Name:      "scene4_back_arm",
				ExtraAttr: []string{"camerabox_facing_back"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{4, pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "scene4_front_arm",
				ExtraAttr: []string{"camerabox_facing_front"},
				ExtraData: []string{pre.CTSVerifierARM.S()},
				Pre:       pre.ITSARMPre,
				Val:       itsParam{4, pb.Facing_FACING_FRONT},
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

	testing.ContextLog(ctx, "Run ITS")
	its := (s.PreValue().(*pre.ITSHelper))
	param := s.Param().(itsParam)
	camID, err := its.CameraID(ctx, param.Facing)
	if err != nil {
		s.Fatalf("Failed to get camera id of camera facing %s: %v", param.Facing, err)
	}
	cmd := its.TestCmd(ctx, param.Scene, camID)

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

	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to get stdin pipe of ITS command: ", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to get stdout pipe of ITS command: ", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.Fatal("Failed to get stderr pipe of ITS command: ", err)
	}
	sceneResults := make(chan *sceneResult)
	itsLogPath := ""
	go func() {
		defer stdin.Close()
		defer stdout.Close()
		defer close(sceneResults)
		var result *sceneResult
		// Rules for parsing lines from stdout/stderr of ITS test with |pat| and run the corresponding |handler|.
		var rules = []struct {
			pat     *regexp.Regexp
			handler func([]string) error
		}{
			// Pattern from http://androidxref.com/9.0.0_r3/xref/cts/apps/CameraITS/tools/run_all_tests.py#241.
			{regexp.MustCompile(`Saving output files to: (\S+)`), func(m []string) error {
				itsLogPath = m[0]
				testing.ContextLog(ctx, "ITS output log: ", itsLogPath)
				return nil
			}},
			// Pattern from http://androidxref.com/9.0.0_r3/xref/cts/apps/CameraITS/tools/run_all_tests.py#337.
			{regexp.MustCompile(`Start running ITS on camera \d+, (scene\d+)`), func(m []string) error {
				result = &sceneResult{name: m[0]}
				testing.ContextLog(ctx, "Start test scene: ", result.name)
				return nil
			}},
			// Pattern from http://androidxref.com/9.0.0_r3/xref/cts/apps/CameraITS/tools/validate_scene.py#48.
			{regexp.MustCompile(`Press Enter after placing camera \d+ to frame the test scene: (\w+)`), func(m []string) error {
				// Scene chart already be displayed as precondition.
				if _, err := stdin.Write([]byte{'\n'}); err != nil {
					return errors.Wrap(err, "failed to confirm that test scene is placed")
				}
				return nil
			}},
			// Pattern from http://androidxref.com/9.0.0_r3/xref/cts/apps/CameraITS/tools/validate_scene.py#66.
			{regexp.MustCompile(`Please check scene setup in`), func(m []string) error {
				if _, err := stdin.Write([]byte{'Y', '\n'}); err != nil {
					return errors.Wrap(err, "failed to confirm that test scene is checked")
				}
				return nil
			}},
			// Pattern from http://androidxref.com/9.0.0_r3/xref/cts/apps/CameraITS/tools/run_all_tests.py#427
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
				if result == nil {
					return errors.New("Test scene result not initialized before writing result")
				}
				result.pass = pass
				result.total = total
				sceneResults <- result
				result = nil
				return nil
			}},
		}

		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			testing.ContextLog(ctx, "[ITS] ", line)
			for _, rule := range rules {
				if m := rule.pat.FindStringSubmatch(line); len(m) != 0 {
					if err := rule.handler(m[1:]); err != nil {
						s.Fatal("Failed to parse ITS stdout, stderr: ", err)
					}
					break
				}
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start ITS: ", err)
	}
	defer func(ctx context.Context) {
		if itsLogPath == "" {
			s.Error("No ITS log path")
			return
		}
		p := path.Join(s.OutDir(), "its")
		if err := testexec.CommandContext(ctx, "mv", itsLogPath, p).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to move ITS log path %v to test output directory: %v", itsLogPath, err)
		}
	}(cleanupCtx)

	for r := range sceneResults {
		if r.total != r.pass {
			s.Errorf("Failed in test scene %v with pass rate %d/%d", r.name, r.pass, r.total)
		}
	}

	if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run ITS: ", err)
	}
}
