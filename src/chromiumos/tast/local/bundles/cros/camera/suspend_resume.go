// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	pb "chromiumos/tast/services/cros/camera"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterSuspendResumeServer(srv, &SuspendResume{s})
		},
	})
}

type SuspendResume struct {
	s *testing.ServiceState
}

func (*SuspendResume) SetUp(ctx context.Context, cfg *pb.SuspendResumeConfig) (*empty.Empty, error) {
	const (
		pkg = "org.chromium.arc.testapp.camerasuspendresume"
		act = pkg + "/.MainActivity"

		testResID    = pkg + ":id/test_result"
		testResLogID = pkg + ":id/test_result_log"
	)
	out := new(empty.Empty)

	apkPath := cfg.GetApkPath()
	testing.ContextLogf(ctx, "Verifying apk exists at %s", apkPath)
	_, err := os.Stat(apkPath)
	if os.IsNotExist(err) {
		return out, errors.Wrap(err, "test apk doesn't exist")
	}

	testing.ContextLog(ctx, "Setting up test environment")
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.KeepState())
	if err != nil {
		return out, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer os.RemoveAll(td)
	a, err := arc.New(ctx, td)
	if err != nil {
		return out, errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return out, errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close()

	testing.ContextLog(ctx, "Installing app and granting needed permission")
	if err := a.Install(ctx, apkPath); err != nil {
		return out, errors.Wrap(err, "failed to install app")
	}

	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.CAMERA").Run(testexec.DumpLogOnError); err != nil {
		return out, errors.Wrap(err, "failed to grant camera permission to test app")
	}

	testing.ContextLog(ctx, "Starting app")
	if err := a.Command(ctx, "am", "start", "-W", act).Run(testexec.DumpLogOnError); err != nil {
		return out, errors.Wrap(err, "failed to start test app")
	}

	testing.ContextLog(ctx, "Waiting for camera to be opened")
	if err := d.Object(ui.ID(testResID), ui.TextContains("1")).WaitForExists(ctx, 20*time.Second); err != nil {
		return out, errors.Wrap(err, "failed to wait for camera to be opened")
	}

	return out, nil
}
