// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterEthernetServiceServer(srv, &EthernetService{s: s})
		},
	})
}

// EthernetService implements tast.cros.network.EthernetService.
type EthernetService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// New logs into a Chrome session as a fake user. Close must be called later
// to clean up the associated resources.
func (e *EthernetService) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if e.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	e.cr = cr
	return &empty.Empty{}, nil
}

// Close releases the resources obtained by New.
func (e *EthernetService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if e.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := e.cr.Close(ctx)
	e.cr = nil
	return &empty.Empty{}, err
}

// DownloadPath returns the download path from cryptohome.
func (e *EthernetService) DownloadPath(ctx context.Context, req *empty.Empty) (*network.DownloadResponse, error) {
	if e.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	downloadsPath, err := cryptohome.DownloadsPath(ctx, e.cr.NormalizedUser())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Downloads path")
	}
	return &network.DownloadResponse{DownloadPath: downloadsPath}, nil
}
