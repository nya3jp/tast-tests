// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package baserpc

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/services/cros/baserpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			baserpc.RegisterFaillogServiceServer(srv, &FaillogService{s: s})
		},
	})
}

// FaillogService implements tast.cros.baserpc.FaillogService.
type FaillogService struct {
	s      *testing.ServiceState
	tmpDir string
}

// Create creates a faillog.tar.gz in target machine.
func (c *FaillogService) Create(ctx context.Context, req *empty.Empty) (*baserpc.CreateResponse, error) {
	// Make sure outdated temporary directory is remove.
	if c.tmpDir != "" {
		os.RemoveAll(c.tmpDir)
	}
	// Create temporary directory.
	var err error
	if c.tmpDir, err = ioutil.TempDir("", ""); err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}

	// Save faillog.
	faillogDir := filepath.Join(c.tmpDir, "faillog")
	if err := os.MkdirAll(faillogDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create faillog in temp dir")
	}
	faillog.SaveToDir(ctx, faillogDir)

	// Return path to faillog as response.
	return &baserpc.CreateResponse{Path: faillogDir}, nil
}

// Remove removes a previous created faillog.tar.gz in target machine.
func (c *FaillogService) Remove(context.Context, *empty.Empty) (*empty.Empty, error) {
	if c.tmpDir == "" {
		return nil, nil
	}
	if err := os.RemoveAll(c.tmpDir); err != nil {
		return nil, err
	}
	c.tmpDir = ""
	return &empty.Empty{}, nil
}
