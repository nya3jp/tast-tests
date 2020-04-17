// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dutfs provides remote file system operations on DUT.
//
// Remote tests usually define their own gRPC services for respective testing
// scenarios, but if tests want to do only a few basic file operations on DUT,
// they can choose to use this package to avoid defining gRPC services.
package dutfs

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/baserpc"
)

// ServiceName is the name of the gRPC service this package uses to access remote
// file system on DUT.
const ServiceName = "tast.cros.baserpc.FileSystem"

// Client provides remote file system operations on DUT.
type Client struct {
	fs baserpc.FileSystemClient
}

// NewClient creates Client from an existing gRPC connection. conn must be
// connected to the cros bundle.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{fs: baserpc.NewFileSystemClient(conn)}
}

// ReadDir reads the directory named by dirname and returns a list of directory
// entries sorted by filename.
func (c *Client) ReadDir(ctx context.Context, dirname string) ([]os.FileInfo, error) {
	res, err := c.fs.ReadDir(ctx, &baserpc.ReadDirRequest{Dir: dirname})
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, decodeErr(res.Error)
	}

	var fis []os.FileInfo
	for _, pb := range res.Files {
		fis = append(fis, fileInfo{pb})
	}
	return fis, nil
}

// Stat returns filesystem status of the file specified by name.
func (c *Client) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	res, err := c.fs.Stat(ctx, &baserpc.StatRequest{Name: name})
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, decodeErr(res.Error)
	}
	return fileInfo{res.Info}, nil
}

// ReadFile reads the file specified by name and returns its contents.
func (c *Client) ReadFile(ctx context.Context, name string) ([]byte, error) {
	res, err := c.fs.ReadFile(ctx, &baserpc.ReadFileRequest{Name: name})
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, decodeErr(res.Error)
	}
	return res.Content, nil
}

// fileInfo wraps baserpc.FileInfo to implement os.FileInfo interface.
type fileInfo struct {
	pb *baserpc.FileInfo
}

var _ os.FileInfo = (*fileInfo)(nil)

func (fi fileInfo) Name() string {
	return fi.pb.Name
}

func (fi fileInfo) Size() int64 {
	return int64(fi.pb.Size)
}

func (fi fileInfo) Mode() os.FileMode {
	return os.FileMode(fi.pb.Mode)
}

func (fi fileInfo) ModTime() time.Time {
	ts, err := ptypes.Timestamp(fi.pb.Modified)
	if err != nil {
		return time.Time{}
	}
	return ts
}

func (fi fileInfo) IsDir() bool {
	return fi.Mode().IsDir()
}

func (fi fileInfo) Sys() interface{} {
	return nil
}

func decodeErr(err *baserpc.Error) error {
	if err == nil {
		return nil
	}

	switch err := err.Type.(type) {
	case *baserpc.Error_Errno:
		return unix.Errno(err.Errno)
	case *baserpc.Error_Link:
		return &os.LinkError{
			Op:  err.Link.Op,
			Old: err.Link.Old,
			New: err.Link.New,
			Err: decodeErr(err.Link.Error),
		}
	case *baserpc.Error_Path:
		return &os.PathError{
			Op:   err.Path.Op,
			Path: err.Path.Path,
			Err:  decodeErr(err.Path.Error),
		}
	case *baserpc.Error_Syscall:
		return &os.SyscallError{
			Syscall: err.Syscall.Syscall,
			Err:     decodeErr(err.Syscall.Error),
		}
	case *baserpc.Error_Msg:
		return errors.New(err.Msg)
	default:
		panic(fmt.Sprintf("decodeErr: unknown type %T", err))
	}
}
