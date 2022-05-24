// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/shill"
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

// Browse browses the url address passed.
func (e *EthernetService) Browse(ctx context.Context, request *network.BrowseRequest) (*empty.Empty, error) {
	if e.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	conn, err := e.cr.NewConn(ctx, request.Url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to chrome")
	}
	if err := conn.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close chrome")
	}
	return &empty.Empty{}, nil
}

// SetWifi enables/disables Wifi via shill.
func (e *EthernetService) SetWifi(ctx context.Context, request *network.WifiRequest) (*empty.Empty, error) {
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Manager object")
	}
	_, err = shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the WiFi interface")
	}
	if request.Enabled {
		if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
			return nil, errors.Wrap(err, "failed to enable wifi via shill")
		}
		return &empty.Empty{}, nil
	}
	if err := manager.DisableTechnology(ctx, shill.TechnologyWifi); err != nil {
		return nil, errors.Wrap(err, "failed to disable wifi via shill")
	}
	return &empty.Empty{}, nil
}
