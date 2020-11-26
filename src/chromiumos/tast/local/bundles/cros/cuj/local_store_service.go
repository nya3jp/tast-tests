// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/cuj"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cuj.RegisterLocalStoreServiceServer(srv, &LocalStoreService{s: s})
		},
	})
}

// LocalStoreService implements tast.cros.cuj.LocalStoreService.
type LocalStoreService struct {
	s      *testing.ServiceState
	tmpDir string
}

// Create creates a temp folder in target machine.
func (c *LocalStoreService) Create(ctx context.Context, req *empty.Empty) (*cuj.CreateResponse, error) {
	// Make sure outdated temporary directory is remove.
	if c.tmpDir != "" {
		os.RemoveAll(c.tmpDir)
	}
	// Create temporary directory.
	var err error
	if c.tmpDir, err = ioutil.TempDir("", ""); err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}

	// Create local storage folder.
	localStorageDir := filepath.Join(c.tmpDir, "local_storage")
	if err := os.MkdirAll(localStorageDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create faillog in temp dir")
	}

	// Return path to faillog as response.
	return &cuj.CreateResponse{Path: localStorageDir}, nil
}

// Remove removes a previous created temp folder in target machine.
func (c *LocalStoreService) Remove(ctx context.Context, req *cuj.RemoveRequest) (*empty.Empty, error) {
	if req.Path == "" {
		return nil, nil
	}
	if err := os.RemoveAll(req.Path); err != nil {
		return nil, err
	}
	req.Path = ""
	return &empty.Empty{}, nil
}
