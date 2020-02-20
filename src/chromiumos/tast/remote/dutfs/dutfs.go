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
	"os"
	"time"

	"github.com/golang/protobuf/ptypes"

	"chromiumos/tast/dut"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/baserpc"
	"chromiumos/tast/testing"
)

// ServiceName is the name of the gRPC service this package uses to access remote
// file system on DUT.
const ServiceName = "tast.cros.baserpc.FileSystem"

// Client provides remote file system operations on DUT.
type Client struct {
	cl *rpc.Client
	fs baserpc.FileSystemClient
}

// Dial establishes a gRPC connection to the DUT for remote filesystem operations.
// Tests calling this function must declare "tast.cros.baserpc.FileSystem" service
// dependency. Close must be called after use to close the gRPC connection.
func Dial(ctx context.Context, d *dut.DUT, h *testing.RPCHint) (*Client, error) {
	cl, err := rpc.Dial(ctx, d, h, "cros")
	if err != nil {
		return nil, err
	}

	fs := baserpc.NewFileSystemClient(cl.Conn)
	return &Client{cl, fs}, nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close(ctx context.Context) error {
	return c.cl.Close(ctx)
}

// ReadDir reads the directory named by dirname and returns a list of directory
// entries sorted by filename.
func (c *Client) ReadDir(ctx context.Context, dirname string) ([]os.FileInfo, error) {
	res, err := c.fs.ReadDir(ctx, &baserpc.ReadDirRequest{Dir: dirname})
	if err != nil {
		return nil, err
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
	return fileInfo{res}, nil
}

// ReadFile reads the file specified by name and returns its contents.
func (c *Client) ReadFile(ctx context.Context, name string) ([]byte, error) {
	res, err := c.fs.ReadFile(ctx, &baserpc.ReadFileRequest{Name: name})
	if err != nil {
		return nil, err
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
