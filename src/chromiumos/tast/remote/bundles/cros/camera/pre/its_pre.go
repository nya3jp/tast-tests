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
	"chromiumos/tast/local/arc"
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
	CTSVerifierX86 CTSVerifier = "android-chs-verifier-9.0_r13-linux_x86-x86.zip"
	// CTSVerifierARM is for test running on ARM compatible platform.
	CTSVerifierARM CTSVerifier = "android-cts-verifier-9.0_r13-linux_x86-arm.zip"
)

// S returns the string form of CTSVerifier.
func (verifier CTSVerifier) S() string {
	return string(verifier)
}

func (verifier CTSVerifier) abi() string {
	if strings.Contains(verifier.S(), "arm") {
		return "arm"
	}
	return "x86"
}

// itsPreImpl implements testing.Precondition.
type itsPreImpl struct {
	verifier   CTSVerifier
	cl         *rpc.Client
	itsCl      pb.ITSServiceClient
	dir        string
	oldEnvPath string
	hostname   string
	adb        *arc.ADB
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

func binPath(ctx context.Context, binName string) (string, error) {
	output, err := testexec.CommandContext(ctx, "which", binName).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get path of binary %v", binName)
	}
	// Trailing newline char is trimmed.
	return strings.TrimSpace(string(output)), nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	content, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, content, perm)
}

func (p *itsPreImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
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

	privateKeyPath := path.Join(p.dir, "testkey")
	p.hostname = d.HostName()
	adb := arc.NewAdbForRemoteTest(privateKeyPath, p.hostname)
	if err := adb.SetUpADBAuth(ctx); err != nil {
		s.Fatal("Failed to set up adb auth: ", err)
	}
	p.adb = adb

	testing.ContextLog(ctx, "ADB connect to DUT")
	if err := adb.ConnectADB(ctx); err != nil {
		s.Fatal("Failed to set up connection to DUT: ", err)
	}

	// Unpack CTSVerifier.
	if err := testexec.CommandContext(
		ctx, "unzip", s.DataPath(p.verifier.S()), "-d", p.dir).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to unpack Cts verifier: ", err)
	}

	// Install CTSVerifier apk.
	verifierAPK := path.Join(p.dir, "android-cts-verifier", "CtsVerifier.apk")
	if err := adb.Command(ctx, "install", "-r", "-g", verifierAPK).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install CTSVerifier: ", err)
	}

	// Set python2 as python.
	py2Path, err := binPath(ctx, "python2")
	if err != nil {
		s.Fatal("Failed to get python2 path: ", err)
	}
	tempPyPath := path.Join(p.dir, "python")
	if err := os.Symlink(py2Path, tempPyPath); err != nil {
		s.Fatalf("Failed to create symlink for python2 path %v: %v", py2Path, err)
	}
	if pyPath, err := binPath(ctx, "python"); err != nil {
		s.Fatal("Failed to get python path: ", err)
	} else if pyPath != tempPyPath {
		s.Fatalf("Failed to hijack python path from %v to %v", pyPath, tempPyPath)
	}

	return &ITSHelper{p}
}

func (p *itsPreImpl) itsRoot() string {
	return path.Join(p.dir, "android-cts-verifier", "CameraITS")
}

func (p *itsPreImpl) Close(ctx context.Context, s *testing.PreState) {
	if err := os.Setenv("PATH", p.oldEnvPath); err != nil {
		s.Errorf("Failed to restore environment variable PATH %v: %v", p.oldEnvPath, err)
	}
	if err := os.RemoveAll(p.dir); err != nil {
		s.Errorf("Failed to remove temp directory %v: %v", p.dir, err)
	}
	if _, err := p.itsCl.TearDown(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to call remote its TearDown(): ", err)
	}
	p.cl.Close(ctx)
}

// TestCmd returns command to run test scene with camera id.
func (h *ITSHelper) TestCmd(ctx context.Context, scene, camera int) *testexec.Cmd {
	setupPath := path.Join("build", "envsetup.sh")
	scriptPath := path.Join("tools", "run_all_tests.py")
	cmdStr := fmt.Sprintf(`cd %s
	source %s
	python %s device=%s scenes=%d camera=%d`,
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
	out, err := h.p.adb.Command(ctx, "shell", "pm", "list", "features").Output(testexec.DumpLogOnError)
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
