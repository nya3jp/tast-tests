// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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

// CTSVerifier contains scripts and test scenes for running Android ITS test
type CTSVerifier struct {
	abi string
	// Zip is the name of different ITS zip downloaded from https://source.android.com/compatibility/cts/downloads.
	Zip string
	// Py3Patches are patches applied on ITS scripts for python3 migration.
	Py3Patches []string
}

// To uprev |ctsVerifierX86Zip| and |ctsVerifierArmZip|, download the new zip
// from https://source.android.com/compatibility/cts/downloads. Check all py2
// to py3 patches here can be applied to test scripts in new zip and can pass
// "camera.ITS.*" tests. In case of patches applied failure, the patches
// require manually update by fixing all patching errors and python2 runtime
// error in test result of "camera.ITS.*". The patch update also require
// reviewed by ITS test owner yinchiayeh@google.com.
const (
	ctsVerifierRoot = "android-cts-verifier"

	// ctsVerifierX86Zip is zip name of test running on x86 compatible platform.
	ctsVerifierX86Zip = "its/x86/android-cts-verifier-9.0_r15-linux_x86-x86.zip"
	// ITSX86CorePy3Patch is the data path of py2 to py3 patch for shared
	// scripts between all scenes for x86 platform.
	ITSX86CorePy3Patch = "its/x86/patch/core.patch"
	// ITSX86Scene0Py3Patch is the data path of py2 to py3 patch for scene
	// 0 test scripts on x86 platform.
	ITSX86Scene0Py3Patch = "its/x86/patch/scene0.patch"

	// ctsVerifierArmZip is zip name of test running on ARM compatible platform.
	ctsVerifierArmZip = "its/x86/android-cts-verifier-9.0_r15-linux_x86-x86.zip"
)

var (
	// CTSVerifierX86 is for test running on x86 compatible platform.
	CTSVerifierX86 = CTSVerifier{"x86", ctsVerifierX86Zip, []string{ITSX86CorePy3Patch, ITSX86Scene0Py3Patch}}
	// CTSVerifierArm is for test running on ARM compatible platform.
	CTSVerifierArm = CTSVerifier{"arm", ctsVerifierArmZip, []string{}}
)

// itsPreImpl implements testing.Precondition.
type itsPreImpl struct {
	verifier   CTSVerifier
	cl         *rpc.Client
	itsCl      pb.ITSServiceClient
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
var ITSX86Pre = &itsPreImpl{verifier: CTSVerifierX86}

// ITSArmPre is the test precondition to run Android x86-arm ITS test.
var ITSArmPre = &itsPreImpl{verifier: CTSVerifierArm}

func (p *itsPreImpl) String() string         { return fmt.Sprintf("its_%s_precondition", p.verifier.abi) }
func (p *itsPreImpl) Timeout() time.Duration { return 5 * time.Minute }

func copyFile(src, dst string, perm os.FileMode) error {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, content, perm)
}

func itsUnzip(ctx context.Context, zipPath, outDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return errors.Wrap(err, "failed to open ITS zip file")
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		src, err := f.Open()
		if err != nil {
			return errors.Wrapf(err, "failed to open file %v in ITS zip", f.Name)
		}
		defer src.Close()
		dstPath := path.Join(outDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return errors.Wrapf(err, "failed to create directory for unzipped ITS file %v", f.Name)
		}
		dst, err := os.Create(dstPath)
		if err != nil {
			return errors.Wrapf(err, "failed to create file for copying ITS file %v", f.Name)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return errors.Wrapf(err, "failed to copy ITS file %v", f.Name)
		}
	}
	return nil
}

func (p *itsPreImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if p.prepared {
		return &ITSHelper{p}
	}

	d := s.DUT()
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
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
	// TODO(b/189714985): Set PATH in CommandContext instead of overriding it here.
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

	// Unpack CTSVerifier.
	if err := itsUnzip(ctx, s.DataPath(p.verifier.Zip), p.dir); err != nil {
		s.Fatal("Failed to unzip: ", err)
	}
	unzipDir := path.Join(p.dir, ctsVerifierRoot)

	// Install CTSVerifier apk.
	verifierAPK := path.Join(unzipDir, "CtsVerifier.apk")
	if err := p.adbDevice.Command(ctx, "install", "-r", "-g", verifierAPK).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install CTSVerifier: ", err)
	}

	// Apply py2 to py3 patches.
	for _, patch := range p.verifier.Py3Patches {
		if err := testexec.CommandContext(
			ctx, "patch", "-d", unzipDir, "-p1", "-i",
			s.DataPath(patch)).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to patch test scripts: ", err)
		}
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
	python3 %s device=%s scenes=%d camera=%d skip_scene_validation`,
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
