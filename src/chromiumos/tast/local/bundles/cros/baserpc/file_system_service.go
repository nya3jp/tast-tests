// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package baserpc

import (
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/services/cros/baserpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			baserpc.RegisterFileSystemServer(srv, &FileSystemService{s})
		},
	})
}

// FileSystemService implements tast.cros.baserpc.FileSystem gRPC service.
type FileSystemService struct {
	s *testing.ServiceState
}

func toFileInfoProto(fi os.FileInfo) (*baserpc.FileInfo, error) {
	ts, err := ptypes.TimestampProto(fi.ModTime())
	if err != nil {
		return nil, err
	}
	return &baserpc.FileInfo{
		Name:     fi.Name(),
		Size:     uint64(fi.Size()),
		Mode:     uint64(fi.Mode()),
		Modified: ts,
	}, nil
}

func (fs *FileSystemService) ReadDir(ctx context.Context, req *baserpc.ReadDirRequest) (*baserpc.ReadDirResponse, error) {
	fis, err := ioutil.ReadDir(req.Dir)
	if err != nil {
		return nil, err
	}

	var res baserpc.ReadDirResponse
	for _, fi := range fis {
		i, err := toFileInfoProto(fi)
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, i)
	}
	return &res, nil
}

func (fs *FileSystemService) Stat(ctx context.Context, req *baserpc.StatRequest) (*baserpc.FileInfo, error) {
	fi, err := os.Stat(req.Name)
	if err != nil {
		return nil, err
	}
	i, err := toFileInfoProto(fi)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (fs *FileSystemService) ReadFile(ctx context.Context, req *baserpc.ReadFileRequest) (*baserpc.ReadFileResponse, error) {
	f, err := ioutil.ReadFile(req.Name)
	if err != nil {
		return nil, err
	}
	return &baserpc.ReadFileResponse{Content: f}, nil
}
