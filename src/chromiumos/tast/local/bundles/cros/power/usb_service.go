// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
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

// GenerateTestFile generates a new temporary test file for testing, with
// provided filename and filesize.
func (u *USBService) GenerateTestFile(ctx context.Context, req *power.TestFileRequest) (*power.TestFileResponse, error) {
	if u.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, u.cr.NormalizedUser())
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve users Downloads path")
	}

	// Source file path.
	sourceFilePath := path.Join(downloadsPath, req.FileName)

	// Create a file with size.
	file, err := os.Create(sourceFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file")
	}
	if req.FileSize == 0 {
		req.FileSize = 1024
	}
	if err := file.Truncate(int64(req.FileSize)); err != nil {
		return nil, errors.Wrapf(err, "failed to truncate file with size %d", req.FileSize)
	}
	return &power.TestFileResponse{Path: sourceFilePath}, nil
}

// FileChecksum checks the checksum for the input file.
func (u *USBService) FileChecksum(ctx context.Context, req *power.TestFileRequest) (*power.TestFileResponse, error) {
	file, err := os.Open(req.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open files")
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return nil, errors.Wrap(err, "failed to calculate the hash of the files")
	}
	return &power.TestFileResponse{FileChecksumValue: h.Sum(nil)}, nil
}

// CopyFile performs copying of file from given source to destination.
func (u *USBService) CopyFile(ctx context.Context, req *power.TestFileRequest) (*empty.Empty, error) {
	sourceFileStat, err := os.Stat(req.SourceFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get file info")
	}

	if !sourceFileStat.Mode().IsRegular() {
		return nil, errors.Errorf("%s is not a regular file", req.SourceFilePath)
	}

	source, err := os.Open(req.SourceFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer source.Close()

	destination, err := os.Create(req.DestinationFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file")
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return nil, errors.Wrap(err, "failed to copy")
	}
	return &empty.Empty{}, nil
}

// RemoveFile will removes given path file.
func (u *USBService) RemoveFile(ctx context.Context, req *power.TestFileRequest) (*empty.Empty, error) {
	if err := os.Remove(req.Path); err != nil {
		return nil, errors.Wrap(err, "failed to remove file")
	}
	return &empty.Empty{}, nil
}
