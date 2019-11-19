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

const (
	pkg = "org.chromium.arc.testapp.camerasuspendresume"
	act = pkg + "/.MainActivity"

	testResID    = pkg + ":id/test_result"
	testResLogID = pkg + ":id/test_result_log"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterSuspendResumeServer(srv, &SuspendResume{s: s})
		},
	})
}

type SuspendResume struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
	td string
	a  *arc.ARC
	d  *ui.Device
}

func (sr *SuspendResume) SetUp(ctx context.Context, cfg *pb.SuspendResumeConfig) (*empty.Empty, error) {
	out := new(empty.Empty)

	apkPath := cfg.GetApkPath()
	testing.ContextLogf(ctx, "Verifying apk exists at %s", apkPath)
	_, err := os.Stat(apkPath)
	if os.IsNotExist(err) {
		return out, errors.Wrap(err, "test apk doesn't exist")
	}

	testing.ContextLog(ctx, "Setting up test environment")
	sr.cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.KeepState())
	if err != nil {
		return out, errors.Wrap(err, "failed to start Chrome")
	}

	sr.td, err = ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	sr.a, err = arc.New(ctx, sr.td)
	if err != nil {
		return out, errors.Wrap(err, "failed to start ARC")
	}

	sr.d, err = ui.NewDevice(ctx, sr.a)
	if err != nil {
		return out, errors.Wrap(err, "failed to initialize UI Automator")
	}

	testing.ContextLog(ctx, "Installing app and granting needed permission")
	if err := sr.a.Install(ctx, apkPath); err != nil {
		return out, errors.Wrap(err, "failed to install app")
	}

	if err := sr.a.Command(ctx, "pm", "grant", pkg, "android.permission.CAMERA").Run(testexec.DumpLogOnError); err != nil {
		return out, errors.Wrap(err, "failed to grant camera permission to test app")
	}

	testing.ContextLog(ctx, "Starting app")
	if err := sr.a.Command(ctx, "am", "start", "-W", act).Run(testexec.DumpLogOnError); err != nil {
		return out, errors.Wrap(err, "failed to start test app")
	}

	testing.ContextLog(ctx, "Waiting for camera to be opened")
	if err := sr.d.Object(ui.ID(testResID), ui.TextContains("1")).WaitForExists(ctx, 20*time.Second); err != nil {
		return out, errors.Wrap(err, "failed to wait for camera to be opened")
	}

	return out, nil
}

func (sr *SuspendResume) TearDown(ctx context.Context, e *empty.Empty) (*empty.Empty, error) {
	out := new(empty.Empty)
	if sr.d != nil {
		sr.d.Close()
	}
	if sr.a != nil {
		sr.a.Close()
	}
	if sr.td != "" {
		os.RemoveAll(sr.td)
	}
	if sr.cr != nil {
		sr.cr.Close(ctx)
	}
	return out, nil
}

func (sr *SuspendResume) ClearResult(ctx context.Context, e *empty.Empty) (*empty.Empty, error) {
	out := new(empty.Empty)
	if err := sr.d.Object(ui.ID(testResID)).SetText(ctx, ""); err != nil {
		return out, errors.Wrap(err, "failed to clear the text in the result field")
	}
	return out, nil
}

func (sr *SuspendResume) GetResult(ctx context.Context, e *empty.Empty) (*pb.SuspendResumeResult, error) {
	res := new(pb.SuspendResumeResult)
	var err error
	res.Result, err = sr.d.Object(ui.ID(testResID)).GetText(ctx)
	if err != nil {
		return res, errors.Wrap(err, "failed to read test result")
	}
	return res, nil
}

func (sr *SuspendResume) GetResultLog(ctx context.Context, e *empty.Empty) (*pb.SuspendResumeResultLog, error) {
	res := new(pb.SuspendResumeResultLog)
	var err error
	res.ResultLog, err = sr.d.Object(ui.ID(testResLogID)).GetText(ctx)
	if err != nil {
		return res, errors.Wrap(err, "failed to read test result")
	}
	return res, nil
}
