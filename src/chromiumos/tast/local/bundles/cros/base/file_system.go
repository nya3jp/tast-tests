// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package base

import (
	"io/ioutil"

	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/services/cros/base"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			base.RegisterFileSystemServer(srv, &FileSystem{s})
		},
	})
}

// FileSystem implements tast.cros.base.FileSystem gRPC service.
type FileSystem struct {
	s *testing.ServiceState
}

func (fs *FileSystem) ReadDir(ctx context.Context, req *base.ReadDirRequest) (*base.ReadDirResponse, error) {
	fis, err := ioutil.ReadDir(req.Dir)
	if err != nil {
		return nil, err
	}

	var res base.ReadDirResponse
	for _, fi := range fis {
		ts, err := ptypes.TimestampProto(fi.ModTime())
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, &base.FileInfo{
			Name:     fi.Name(),
			Size:     uint64(fi.Size()),
			Mode:     uint64(fi.Mode()),
			Modified: ts,
		})
	}
	return &res, nil
}
