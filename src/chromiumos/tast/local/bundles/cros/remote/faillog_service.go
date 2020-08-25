// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package remote

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/services/cros/remote"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			remote.RegisterFaillogServiceServer(srv, &FaillogService{s: s})
		},
	})
}

// FaillogService implements tast.cros.remote.FaillogService.
type FaillogService struct {
	s      *testing.ServiceState
	tmpDir string
}

// Create creates a faillog.tar.gz in target machine
func (c *FaillogService) Create(ctx context.Context, req *empty.Empty) (*remote.CreateResponse, error) {
	if c.tmpDir != "" {
		os.RemoveAll(c.tmpDir)
	}

	var err error
	if c.tmpDir, err = ioutil.TempDir("", ""); err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	faillog.SaveToDir(ctx, c.tmpDir)
	faillogDir := filepath.Join(c.tmpDir, "faillog")
	faillogTar := filepath.Join(c.tmpDir, "faillog.tar.gz")

	if err := exec.Command("tar", "-cvzf", faillogTar, "-C", c.tmpDir, "faillog").Run(); err != nil {
		return nil, err
	}
	os.RemoveAll(faillogDir)

	return &remote.CreateResponse{Path: faillogTar}, nil
}

// Remove remove a previous crated faillog.tar.gz in target machine
func (c *FaillogService) Remove(context.Context, *empty.Empty) (*empty.Empty, error) {
	if c.tmpDir == "" {
		return nil, nil
	}
	if err := os.RemoveAll(c.tmpDir); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
