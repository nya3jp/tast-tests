// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package baserpc

import (
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
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

// ReadDir returns a list of files in a directory.
func (fs *FileSystemService) ReadDir(ctx context.Context, req *baserpc.ReadDirRequest) (*baserpc.ReadDirResponse, error) {
	var res baserpc.ReadDirResponse
	res.Error = encodeErr(func() error {
		fis, err := ioutil.ReadDir(req.Dir)
		if err != nil {
			return err
		}

		for _, fi := range fis {
			i, err := toFileInfoProto(fi)
			if err != nil {
				return err
			}
			res.Files = append(res.Files, i)
		}
		return nil
	}())
	return &res, nil
}

// Stat returns information of a file.
func (fs *FileSystemService) Stat(ctx context.Context, req *baserpc.StatRequest) (*baserpc.StatResponse, error) {
	var res baserpc.StatResponse
	res.Error = encodeErr(func() error {
		fi, err := os.Stat(req.Name)
		if err != nil {
			return err
		}
		i, err := toFileInfoProto(fi)
		if err != nil {
			return err
		}
		res.Info = i
		return nil
	}())
	return &res, nil
}

// ReadFile reads the content of a file.
func (fs *FileSystemService) ReadFile(ctx context.Context, req *baserpc.ReadFileRequest) (*baserpc.ReadFileResponse, error) {
	var res baserpc.ReadFileResponse
	res.Error = encodeErr(func() error {
		f, err := ioutil.ReadFile(req.Name)
		if err != nil {
			return err
		}
		res.Content = f
		return nil
	}())
	return &res, nil
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

func encodeErr(err error) *baserpc.Error {
	switch err := err.(type) {
	case unix.Errno:
		return &baserpc.Error{Type: &baserpc.Error_Errno{Errno: uint32(err)}}
	case *os.LinkError:
		return &baserpc.Error{Type: &baserpc.Error_Link{Link: &baserpc.LinkError{
			Op:    err.Op,
			Old:   err.Old,
			New:   err.New,
			Error: encodeErr(err.Err),
		}}}
	case *os.PathError:
		return &baserpc.Error{Type: &baserpc.Error_Path{Path: &baserpc.PathError{
			Op:    err.Op,
			Path:  err.Path,
			Error: encodeErr(err.Err),
		}}}
	case *os.SyscallError:
		return &baserpc.Error{Type: &baserpc.Error_Syscall{Syscall: &baserpc.SyscallError{
			Syscall: err.Syscall,
			Error:   encodeErr(err.Err),
		}}}
	case nil:
		return nil
	default:
		return &baserpc.Error{Type: &baserpc.Error_Msg{Msg: err.Error()}}
	}
}
