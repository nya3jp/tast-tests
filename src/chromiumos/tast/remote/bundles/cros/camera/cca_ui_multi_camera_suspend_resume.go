// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/camera"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMultiCameraSuspendResume,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functionality of multi-camera after suspend-resume scenario",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Vars:         []string{"servo"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"cca_ui.js"},
		ServiceDeps:  []string{"tast.cros.camera.CCAService"},
		HardwareDeps: hwdep.D(hwdep.X86()),
		Params: []testing.Param{
			{
				Name: "user_facing_photo",
				Val:  &camera.CameraTestRequest{Mode: camera.CameraMode_PHOTO, Facing: camera.Facing_FACING_FRONT},
			},
			{
				Name: "env_facing_photo",
				Val:  &camera.CameraTestRequest{Mode: camera.CameraMode_PHOTO, Facing: camera.Facing_FACING_BACK},
			},
			{
				Name: "user_facing_video",
				Val:  &camera.CameraTestRequest{Mode: camera.CameraMode_VIDEO, Facing: camera.Facing_FACING_FRONT},
			},
			{
				Name: "env_facing_video",
				Val:  &camera.CameraTestRequest{Mode: camera.CameraMode_VIDEO, Facing: camera.Facing_FACING_BACK},
			},
		},
	})
}

func CCAUIMultiCameraSuspendResume(ctx context.Context, s *testing.State) {
	const (
		slpS0Cmd     = "cat /sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateCmd = "cat /sys/kernel/debug/pmc_core/package_cstate_show"
	)

	// Set up Servo in remote tests.
	dut := s.DUT()
	testRequest := s.Param().(*camera.CameraTestRequest)
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Connect to RPC.
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	cr := camera.NewCCAServiceClient(cl.Conn)

	if _, err := cr.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.CloseChrome(ctx, &empty.Empty{})

	scriptContent, err := ioutil.ReadFile(s.DataPath("cca_ui.js"))
	if err != nil {
		s.Fatal("Failed to load camera script: ", err)
	}
	testRequest.ScriptContents = [][]byte{scriptContent}

	_, err = cr.OpenCamera(ctx, testRequest)
	if err != nil {
		s.Fatal("Failed to open the camera: ", err)
	}
	defer cr.CloseCamera(ctx, &empty.Empty{})

	if testRequest.Mode == camera.CameraMode_PHOTO {
		_, err := cr.TakePicture(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to take picture: ", err)
		}
	} else if testRequest.Mode == camera.CameraMode_VIDEO {
		_, err := cr.RecordVideo(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to record video: ", err)
		}
	}

	cmdOutput := func(cmd string) string {
		out, err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatalf("Failed to execute command %q: %v", cmd, err)
		}
		return strings.Trim(string(out), "\n")
	}

	// SLP output before lid close.
	slpBeforeSuspend := cmdOutput(slpS0Cmd)
	pkgCBeforeSuspend := cmdOutput(pkgCstateCmd)
	c10PkgPattern := regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
	matchSetPre := (c10PkgPattern).FindStringSubmatch(pkgCBeforeSuspend)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgCBeforeSuspend)
	}
	pkgCStateBeforeSuspend := matchSetPre[1]

	s.Log("Closing lid")
	if err := pxy.Servo().SetString(ctx, "lid_open", "no"); err != nil {
		s.Fatal("Failed to close lid: ", err)
	}
	if err := dut.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait DUT to become unreachable: ", err)
	}
	s.Log("Opening lid")
	if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
		s.Fatal("Failed to open lid: ", err)
	}
	if err := dut.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to wake up DUT: ", err)
	}

	slpAfterSuspend := cmdOutput(slpS0Cmd)
	if slpBeforeSuspend == slpAfterSuspend {
		s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", slpBeforeSuspend, slpAfterSuspend)
	}
	if slpAfterSuspend == "0" {
		s.Fatal("Failed SLP counter value must be non-zero, noted is: ", slpAfterSuspend)
	}
	pkgCAfterSuspend := cmdOutput(pkgCstateCmd)
	matchSetPost := (c10PkgPattern).FindStringSubmatch(pkgCAfterSuspend)
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgCAfterSuspend)
	}
	pkgCStateAfterSuspend := matchSetPost[1]
	if pkgCStateBeforeSuspend == pkgCStateAfterSuspend {
		s.Fatalf("Failed Package C10 value %q must be different than value noted earlier %q", pkgCStateBeforeSuspend, pkgCStateAfterSuspend)
	}
	if pkgCStateAfterSuspend == "0x0" || pkgCStateAfterSuspend == "0" {
		s.Fatal("Failed Package C10 should be non-zero")
	}

	cl, err = rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	cr = camera.NewCCAServiceClient(cl.Conn)

	if _, err := cr.ReuseChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to reconnect to the Chrome session: ", err)
	}

	s.Log("Check Camera is active after suspend resume")
	testResponse, err := cr.CheckCameraExists(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to check for active camera: ", err)
	}

	// Check test result.
	switch testResponse.Result {
	case camera.TestResult_TEST_RESULT_PASSED:
		s.Log("Remote test passed")
	case camera.TestResult_TEST_RESULT_FAILED:
		s.Error("Remote test failed with error message:", testResponse.Error)
	case camera.TestResult_TEST_RESULT_UNSET:
		s.Error("Remote test result is unset")
	}

	defer func() {
		s.Log("Opening lid")
		if err := pxy.Servo().SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
	}()
}
