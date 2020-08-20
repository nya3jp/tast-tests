// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// Service includes object would used in gRPC
type Service struct {
	chrome.SvcLoginReusePre
}

// TestAPIConn wraps `TestAPIConn` method under chrome package for gRPC remote call
func (s *Service) TestAPIConn(ctx context.Context) (*chrome.TestConn, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	conn, err := s.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}
	return conn, nil
}

// NewConn wraps `NewConn` method under chrome package for gRPC remote call
func (s *Service) NewConn(ctx context.Context, url string) (*chrome.Conn, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	conn, err := s.CR.NewConn(ctx, url)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeCDPTgt, err, url)
	}
	return conn, nil
}

// New creates new gRPC service
func New(s *testing.ServiceState) Service {
	return Service{chrome.SvcLoginReusePre{S: s}}
}
