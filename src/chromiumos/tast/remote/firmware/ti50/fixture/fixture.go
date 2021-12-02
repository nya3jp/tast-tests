// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture provides ti50 devboard related fixtures.
package fixture

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/common/firmware/ti50"
	remoteTi50 "chromiumos/tast/remote/firmware/ti50"
	"chromiumos/tast/testing"
)

const (
	// DevBoardService is the arg name for the service's host:port pair.
	DevBoardService = "devboardsvc"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:         DevBoardService,
		Desc:         "The DevBoard is connected a devboardsvc service host",
		Contacts:     []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:         &impl{},
		Vars:         []string{DevBoardService},
		ResetTimeout: 5 * time.Second,
	})

}

// Value allows tests to obtain a ti50 devboard.
type Value struct {
	hostPort string
	devboard *remoteTi50.DUTControlAndreiboard
	grpcConn *grpc.ClientConn
}

// DevBoard connects to devboardsvc server and returns the DevBoard instance.
func (v *Value) DevBoard(ctx context.Context, bufLen int, readTimeout time.Duration) (ti50.DevBoard, error) {
	if v.grpcConn == nil {
		conn, err := grpc.DialContext(ctx, v.hostPort, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		v.grpcConn = conn
	}
	if v.devboard != nil {
		v.devboard.Close(ctx)
	}
	v.devboard = remoteTi50.NewDUTControlAndreiboard(v.grpcConn, bufLen, readTimeout)
	err := v.devboard.Reset(ctx)
	if err != nil {
		return nil, err
	}
	return v.devboard, nil
}

type impl struct {
	v *Value
}

func (i *impl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	hostPort := s.RequiredVar(DevBoardService)
	i.v = &Value{hostPort: hostPort}
	return i.v
}

func (i *impl) Reset(ctx context.Context) error {
	if i.v.devboard != nil {
		return i.v.devboard.Reset(ctx)
	}
	return nil
}

func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	if i.v.devboard != nil {
		i.v.devboard.Close(ctx)
		i.v.devboard = nil
	}
}

func (i *impl) String() string {
	return DevBoardService
}
