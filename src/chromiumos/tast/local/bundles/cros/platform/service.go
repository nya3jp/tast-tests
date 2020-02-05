// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains RPC wrappers to set up and tear down tests.
package platform

import (
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	platformCrash "chromiumos/tast/local/bundles/cros/platform/crash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/filterrpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			filterrpc.RegisterFilterServiceServer(srv, &FilterService{s: s})
		},
	})
}

// FilterService implements tast.cros.crash.FilterService
type FilterService struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (c *FilterService) EnableCrashFiltering(ctx context.Context, req *filterrpc.EnableCrashFilteringRequest) (*empty.Empty, error) {
	return &empty.Empty{}, platformCrash.EnableCrashFiltering(req.Name)
}

func (c *FilterService) ReadFile(ctx context.Context, req *filterrpc.ReadFileRequest) (*filterrpc.ReadFileResponse, error) {
	f, err := os.Open(req.Path)
	if err != nil {
		return nil, err
	}
	var d []byte
	for {
		var buf []byte
		n, err := f.Read(buf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		d = append(d, buf...)
	}
	var out filterrpc.ReadFileResponse
	out.Data = string(d)
	return &out, nil
}
