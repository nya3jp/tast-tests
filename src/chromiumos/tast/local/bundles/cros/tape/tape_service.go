// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			tape.RegisterTapeServiceServer(srv, &TapeService{s: s})
		},
	})
}

// TapeService implements tast.cros.tape.TapeService.
type TapeService struct { // NOLINT
	s         *testing.ServiceState
	tokenDir  string
	tokenFile string
	token     string
}

// CreateTokenFile creates a directory. It needs to be removed with RemoveTokenDir.
func (c *TapeService) CreateTokenFile(ctx context.Context, req *tape.CreateTokenFileRequest) (*empty.Empty, error) {

	// Remove existing data.
	os.RemoveAll(req.Path)

	if err := os.MkdirAll(req.Path, 0755); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create token directory")
	}

	file, err := os.Create(filepath.Join(req.Path, req.File))
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create token file")
	}
	defer file.Close()

	if _, err := file.WriteString(req.Token); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to write token to file")
	}

	return &empty.Empty{}, nil
}

// RemoveTokenFile removes a directory created with CreateTokenDir.
func (c *TapeService) RemoveTokenFile(ctx context.Context, req *tape.RemoveTokenFileRequest) (*empty.Empty, error) {
	if err := os.RemoveAll(req.Path); err != nil {
		return &empty.Empty{}, errors.Wrapf(err, "failed to remove %q", req.Path)
	}

	return &empty.Empty{}, nil
}
