// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"path"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/services/cros/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			power.RegisterUSBServiceServer(srv, &USBService{s: s})
		},
	})
}

// USBService implements tast.cros.power.USBService.
type USBService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// NewChrome logs into a Chrome session as a fake user. Close must be called later
// to clean up the associated resources.
func (u *USBService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr != nil {
		return nil, errors.New("Chrome already available")
	}
	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	u.cr = cr
	return &empty.Empty{}, nil
}

// CloseChrome releases the resources obtained by New.
func (u *USBService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := u.cr.Close(ctx)
	u.cr = nil
	return &empty.Empty{}, err
}

// ReuseChrome passes an Option to New to make Chrome reuse the existing login session.
func (u *USBService) ReuseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx, chrome.TryReuseSession())
	if err != nil {
		return nil, err
	}
	u.cr = cr
	return &empty.Empty{}, nil
}

// USBMountPaths returns the mount paths for USB.
func (u *USBService) USBMountPaths(ctx context.Context, req *empty.Empty) (*power.MountPathResponse, error) {
	var MountPaths []string
	info, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to mount info")
	}
	for _, i := range info {
		if strings.HasPrefix(i.MountPath, "/media/removable") {
			MountPaths = append(MountPaths, i.MountPath)
		}
	}
	if len(MountPaths) == 0 {
		return nil, errors.New("no mount path found")
	}
	return &power.MountPathResponse{MountPaths: MountPaths}, nil
}

// GenerateTestFile generates a new temporary test file for testing.
func (u *USBService) GenerateTestFile(ctx context.Context, req *power.TestFileRequest) (*power.TestFileResponse, error) {
	sourcePath, err := ioutil.TempDir("", "temp")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp directory")
	}

	// Source file path.
	sourceFilePath := path.Join(sourcePath, req.FileName)

	if err := ioutil.WriteFile(sourceFilePath, []byte("test"), 0644); err != nil {
		return nil, errors.Wrap(err, "failed to create test file in tempdir")
	}
	return &power.TestFileResponse{Path: sourceFilePath}, nil
}
