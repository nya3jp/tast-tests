// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/proxy"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterProxyServiceServer(srv, &ProxyService{s: s})
		},
	})
}

// ProxyService implements the tast.cros.network.ProxyService gRPC service.
type ProxyService struct {
	s     *testing.ServiceState
	proxy *proxy.Server
}

// StartServer starts a new proxy server instance with a specific configuration.
// This is the implementation of network.ProxyService/Start gRPC.
func (s *ProxyService) StartServer(ctx context.Context, request *network.StartServerRequest) (*network.StartServerResponse, error) {
	s.proxy = proxy.NewServer()

	var cred *proxy.AuthCredentials

	if auth := request.AuthCredentials; auth != nil && auth.Username != "" && auth.Password != "" {
		cred = &proxy.AuthCredentials{
			Username: auth.Username,
			Password: auth.Password,
		}
	}
	var port = 3128
	if request.Port != 0 {
		port = int(request.Port)
	}

	if err := s.proxy.Start(ctx, port, cred); err != nil {
		return nil, errors.Wrap(err, "failed to setup proxy server")
	}

	return &network.StartServerResponse{
		HostAndPort: s.proxy.HostAndPort,
	}, nil
}

// StopServer stops a previously started server instance. Returns an error if no proxy server instance was started on the DUT.
// This is the implementation of network.ProxyService/Stop gRPC.
func (s *ProxyService) StopServer(ctx context.Context, request *empty.Empty) (*empty.Empty, error) {
	if s.proxy == nil {
		return nil, errors.New("no proxy server instance was started")
	}

	if err := s.proxy.Stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop proxy server")
	}
	return &empty.Empty{}, nil
}
