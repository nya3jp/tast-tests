// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	remoteadb "chromiumos/tast/remote/android/adb"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

// To uprev |ctsVerifierX86Zip| and |ctsVerifierArmZip|, download the new zip
// from https://source.android.com/compatibility/cts/downloads, replace old zip
// under data folder and check the test can still pass.
const (
	ctsVerifierRoot = "android-cts-verifier"

	// CtsVerifierX86Zip is data path to ITS bundle testing x86 compatible platform.
	CtsVerifierX86Zip = "its/android-cts-verifier-9.0_r15-linux_x86-x86.zip"

	// CtsVerifierArmZip is data path to ITS bundle testing arm compatible platform.
	CtsVerifierArmZip = "its/x86/android-cts-verifier-9.0_r15-linux_x86-x86.zip"

	// ITSX86CorePy3Patch is the data path of py2 to py3 patch for scripts
	// shared between all scenes for x86 platform. Update the script
	// content with the steps:
	// $ python3 unpack_bundle.py android-cts-verifier-XXX.zip
	// $ cd android-cts-verifier/CameraITS
	// # Do modification to *.py
	// $ git diff base > <Path to this patch>
	ITSPy3Patch = "its/its.patch"

	// UnpackITSBundleScript is the data path of script unpacking ITS
	// bundle and apply python3 patches.
	UnpackITSBundleScript = "its/unpack_bundle.py"
)

type bundleAbi string

const (
	x86 bundleAbi = "x86"
	arm           = "arm"
)

func (abi bundleAbi) bundlePath() (string, error) {
	switch abi {
	case x86:
		return CtsVerifierX86Zip, nil
	case arm:
		return CtsVerifierArmZip, nil
	}
	return "", errors.Errorf("cannot get bundle path unknown abi %v", abi)
}

// itsPreImpl implements testing.Precondition.
type itsPreImpl struct {
	cl         *rpc.Client
	itsCl      pb.ITSServiceClient
	abi        bundleAbi
	dir        string
	oldEnvPath string
	hostname   string
	adbDevice  *adb.Device
	prepared   bool
}

// ITSHelper provides helper functions accessing ITS package and mandating ARC.
type ITSHelper struct {
	p *itsPreImpl
}

// ITSX86Pre is the test precondition to run Android x86 ITS test.
var ITSX86Pre = &itsPreImpl{abi: x86}

// ITSArmPre is the test precondition to run Android x86-arm ITS test.
var ITSArmPre = &itsPreImpl{abi: arm}

func (p *itsPreImpl) String() string         { return fmt.Sprintf("its_%s_precondition", p.abi) }
func (p *itsPreImpl) Timeout() time.Duration { return 5 * time.Minute }

func copyFile(src, dst string, perm os.FileMode) error {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, content, perm)
}

func (p *itsPreImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if p.prepared {
		return &ITSHelper{p}
	}

	d := s.DUT()
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	p.cl = cl

	// Set up ARC on DUT.
	itsClient := pb.NewITSServiceClient(cl.Conn)
	_, err = itsClient.SetUp(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Remote call Setup() failed: ", err)
	}
	p.itsCl = itsClient

	// Prepare temp bin dir.
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temp dir for extra binaries: ", err)
	}
	p.dir = tempDir
	p.oldEnvPath = os.Getenv("PATH")
	os.Setenv("PATH", p.dir+":"+p.oldEnvPath)

	// Prepare ADB downloaded from fixed url without versioning (Same
	// strategy as CTS), may consider associate proper version in
	// tast-build-deps.
	if err := copyFile(s.DataPath("adb"), path.Join(p.dir, "adb"), 0755); err != nil {
		s.Fatal("Failed to copy adb binary: ", err)
	}

	p.hostname = d.HostName()
	if err := remoteadb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	testing.ContextLog(ctx, "ADB connect to DUT")
	adbDevice, err := adb.Connect(ctx, p.hostname, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to set up adb connection to DUT: ", err)
	}
	p.adbDevice = adbDevice

	// Unpack ITS bundle.
	bundlePath, err := p.abi.bundlePath()
	if err != nil {
		s.Fatal("Failed to get bundle path: ", err)
	}
	if err := testexec.CommandContext(
		ctx, "python3", s.DataPath(UnpackITSBundleScript), s.DataPath(bundlePath),
		"--patch_path", s.DataPath(ITSPy3Patch), "--output", tempDir).Run(); err != nil {
		s.Fatal("Failed to unpack bundle path: ", err)
	}

	// Install CTSVerifier apk.
	ctsVerifierRootPath := path.Join(p.dir, ctsVerifierRoot)
	verifierAPK := path.Join(ctsVerifierRootPath, "CtsVerifier.apk")
	if err := p.adbDevice.Command(ctx, "install", "-r", "-g", verifierAPK).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install CTSVerifier: ", err)
	}

	p.prepared = true
	return &ITSHelper{p}
}

func (p *itsPreImpl) itsRoot() string {
	return path.Join(p.dir, ctsVerifierRoot, "CameraITS")
}

func (p *itsPreImpl) Close(ctx context.Context, s *testing.PreState) {
	if len(p.oldEnvPath) > 0 {
		if err := os.Setenv("PATH", p.oldEnvPath); err != nil {
			s.Errorf("Failed to restore environment variable PATH %v: %v", p.oldEnvPath, err)
		}
	}
	if len(p.dir) > 0 {
		if err := os.RemoveAll(p.dir); err != nil {
			s.Errorf("Failed to remove temp directory %v: %v", p.dir, err)
		}
	}
	if p.itsCl != nil {
		if _, err := p.itsCl.TearDown(ctx, &empty.Empty{}); err != nil {
			s.Error("Failed to call remote its TearDown(): ", err)
		}
	}
	if p.cl != nil {
		p.cl.Close(ctx)
	}
	p.prepared = false
}

// TestCmd returns command to run test scene with camera id.
func (h *ITSHelper) TestCmd(ctx context.Context, scene, camera int) *testexec.Cmd {
	setupPath := path.Join("build", "envsetup.sh")
	scriptPath := path.Join("tools", "run_all_tests.py")
	cmdStr := fmt.Sprintf(`cd %s
	source %s
	python %s device=%s scenes=%d camera=%d skip_scene_validation`,
		h.p.itsRoot(), setupPath, scriptPath, h.p.hostname, scene, camera)
	cmd := testexec.CommandContext(ctx, "bash", "-c", cmdStr)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=y")
	return cmd
}

// Chart returns scene chart to run test scene.
func (h *ITSHelper) Chart(scene int) string {
	s := fmt.Sprintf("scene%d", scene)
	return path.Join(h.p.itsRoot(), "tests", s, s+".pdf")
}

// CameraID returns corresponding camera id of camera facing on DUT.
func (h *ITSHelper) CameraID(ctx context.Context, facing pb.Facing) (int, error) {
	out, err := h.p.adbDevice.Command(ctx, "shell", "pm", "list", "features").Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, errors.Wrap(err, "failed to list features on ARC")
	}
	var front, back bool
	for _, feature := range strings.Split(string(out), "\n") {
		if feature == "feature:android.hardware.camera.front" {
			front = true
		} else if feature == "feature:android.hardware.camera" {
			back = true
		}
	}
	if (facing == pb.Facing_FACING_BACK && !back) || (facing == pb.Facing_FACING_FRONT && !front) {
		return -1, errors.Errorf("cannot run test on DUT without %s facing camera", facing)
	}
	if back && front && facing != pb.Facing_FACING_BACK {
		return 1, nil
	}
	return 0, nil
}
