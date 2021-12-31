// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var screenRecorderService ScreenRecorderService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			screenRecorderService = ScreenRecorderService{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterScreenRecorderServiceServer(srv, &screenRecorderService)
		},
		GuaranteeCompatibility: true,
	})
}

// ScreenRecorderService implements tast.cros.ui.ScreenRecorderService
// It provides functionalities to perform screen recording.
type ScreenRecorderService struct {
	sharedObject   *common.SharedObjectsForService
	mutex          sync.Mutex
	screenRecorder *uiauto.ScreenRecorder
	fileName       string
}

// Start creates a new media recorder and starts to record the screen.
// There can be only a single recording in progress at a time.
// If user does not specify the file name, the service will generate a
// temporary location for the recording and return that to the user in Stop().
func (svc *ScreenRecorderService) Start(ctx context.Context, req *pb.StartRequest) (*empty.Empty, error) {
	//Follow the same locking sequence from svc lock to shared object lock to avoid deadlock
	svc.mutex.Lock()
	defer svc.mutex.Unlock()
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if svc.screenRecorder != nil {
		return nil, errors.New("cannot start again when recording is in progress")
	}

	cr := svc.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// TODO(b/219091883): The underlying screen recording API leverages on screen navigation to start
	// the Screen recording. Screen recording is limited to the time when the user is logged
	// in and does not go beyond the boundary of a chrome session. Research to see recording
	// can span across chrome sessions.
	svc.screenRecorder, err = uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil || svc.screenRecorder == nil {
		return nil, errors.Wrap(err, "failed to create ScreenRecorder")
	}
	if req.FileName != "" {
		svc.fileName = req.FileName
	} else {
		svc.fileName = ""
	}
	svc.screenRecorder.Start(ctx, tconn)

	return &empty.Empty{}, nil
}

// Stop stops and saves the recording to the specified location.
func (svc *ScreenRecorderService) Stop(ctx context.Context, req *empty.Empty) (*pb.StopResponse, error) {
	svc.mutex.Lock()
	defer svc.mutex.Unlock()
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	if svc.screenRecorder == nil {
		return nil, errors.New("failed to stop when no recording is progress")
	}

	var fileName string
	if svc.fileName == "" {
		// Create a temporary file if user does not give a specific path
		tempFile, err := ioutil.TempFile("", "record*.webm")
		if err != nil {
			return nil, err
		}
		fileName = tempFile.Name()
	} else {
		// Ensure that parent directories of the provided path are created
		fileName = svc.fileName
		dir := path.Dir(fileName)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, errors.Wrap(err, "failed to create a dir at "+dir)
		}
	}

	uiauto.ScreenRecorderStopSaveRelease(ctx, svc.screenRecorder, fileName)
	svc.screenRecorder = nil
	svc.fileName = ""
	return &pb.StopResponse{FileName: fileName}, nil
}
