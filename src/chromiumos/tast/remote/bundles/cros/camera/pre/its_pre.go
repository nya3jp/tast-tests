// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pre

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

// CTSVerifier contains scripts and test scenes for running Android ITS test
// and is downloaded from https://source.android.com/compatibility/cts/downloads.
type CTSVerifier string

const (
	// CTSVerifierX86 is for test running on x86 compatible platform.
	CTSVerifierX86 CTSVerifier = "android-cts-verifier-9.0_r15-linux_x86-x86.zip"
	// CTSVerifierARM is for test running on ARM compatible platform.
	CTSVerifierARM CTSVerifier = "android-cts-verifier-9.0_r15-linux_x86-arm.zip"
)

// DataPath returns data path of the zip file.
func (v CTSVerifier) DataPath() string {
	return fmt.Sprintf("its/%s/%s", v.abi(), v)
}

// PatchesDataPathes returns data path of patches to be applied.
func (v CTSVerifier) PatchesDataPathes() []string {
	patch := func(name string) string {
		return fmt.Sprintf("its/%s/patch/%s.patch", v.abi(), name)
	}
	return []string{patch("core"), patch("scene0")}
}

func (v CTSVerifier) abi() string {
	if string(v) == string(CTSVerifierX86) {
		return "x86"
	}
	return "arm"
}

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

// ITSHelper provides helper functions accessing ITS package and mandating ARC++.
type ITSHelper struct {
	p *itsPreImpl
}

// ITSX86Pre is the test precondition to run Android x86 ITS test.
var ITSX86Pre = &itsPreImpl{verifier: CTSVerifierX86}

// ITSARMPre is the test precondition to run Android x86-arm ITS test.
var ITSARMPre = &itsPreImpl{verifier: CTSVerifierARM}

func (p *itsPreImpl) String() string         { return fmt.Sprintf("its_%s_precondition", p.verifier.abi()) }
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
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	p.cl = cl

	// Set up ARC++ on DUT.
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

	// Prepare ADB.
	if err := copyFile(s.DataPath("adb"), path.Join(p.dir, "adb"), 0755); err != nil {
		s.Fatal("Failed to copy adb binary: ", err)
	}

	p.hostname = d.HostName()
	if err := adb.LaunchServer(ctx); err != nil {
		s.Fatal("Failed to launch adb server: ", err)
	}

	testing.ContextLog(ctx, "ADB connect to DUT")
	adbDevice, err := adb.Connect(ctx, p.hostname, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to set up adb connection to DUT: ", err)
	}
	p.adbDevice = adbDevice

	// Unpack CTSVerifier.
	if err := testexec.CommandContext(
		ctx, "unzip", s.DataPath(p.verifier.DataPath()), "-d", p.dir).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to unpack Cts verifier: ", err)
	}
	unzipDir := path.Join(p.dir, "android-cts-verifier")

	// Install CTSVerifier apk.
	verifierAPK := path.Join(unzipDir, "CtsVerifier.apk")
	if err := p.adbDevice.Command(ctx, "install", "-r", "-g", verifierAPK).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install CTSVerifier: ", err)
	}

	// Apply py2 to py3 patches.
	for _, p := range p.verifier.PatchesDataPathes() {
		if err := testexec.CommandContext(
			ctx, "patch", "-d", unzipDir, "-p1", "-i",
			s.DataPath(p)).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to patch test scripts: ", err)
		}
	}

	p.prepared = true
	return &ITSHelper{p}
}

func (p *itsPreImpl) itsRoot() string {
	return path.Join(p.dir, "android-cts-verifier", "CameraITS")
}

func (p *itsPreImpl) Close(ctx context.Context, s *testing.PreState) {
	if err := os.Setenv("PATH", p.oldEnvPath); err != nil {
		s.Errorf("Failed to restore environment variable PATH %v: %v", p.oldEnvPath, err)
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
	s := "scene" + strconv.Itoa(scene)
	return path.Join(h.p.itsRoot(), "tests", s, s+".pdf")
}

// CameraID returns corresponding camera id of camera facing on DUT.
func (h *ITSHelper) CameraID(ctx context.Context, facing pb.Facing) (int, error) {
	out, err := h.p.adbDevice.Command(ctx, "shell", "pm", "list", "features").Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, errors.Wrap(err, "failed to list features on ARC++")
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
	if back && front {
		if facing == pb.Facing_FACING_BACK {
			return 0, nil
		}
		return 1, nil
	}
	return 0, nil
}
