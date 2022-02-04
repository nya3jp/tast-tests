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
	// DevBoardService arg name for the service's host:port pair and also the name of the fixture.
	DevBoardService = "devboardsvc"

	// Ti50 fixture flashes a ti50 image.
	Ti50 = "ti50"

	// SystemTestAuto fixture flashes a system_test_auto image.
	SystemTestAuto = "systemTestAuto"

	setUpTimeout    = 2 * time.Minute
	resetTimeout    = 5 * time.Second
	tearDownTimeout = 5 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            DevBoardService,
		Desc:            "A DevBoard connected to a devboardsvc service host",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            &impl{},
		Vars:            []string{DevBoardService},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: tearDownTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name:            Ti50,
		Desc:            "Uses devboardsvc to flash a Ti50 image",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            &impl{},
		Vars:            []string{DevBoardService},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: tearDownTimeout,
		Parent:          Ti50Image,
	})
	testing.AddFixture(&testing.Fixture{
		Name:            SystemTestAuto,
		Desc:            "Uses devboardsvc to flash a system_test_auto image",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            &impl{},
		Vars:            []string{DevBoardService},
		SetUpTimeout:    setUpTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: tearDownTimeout,
		Parent:          SystemTestAutoImage,
	})
}

// Value allows tests to obtain a ti50 devboard.
type Value struct {
	grpcConn *grpc.ClientConn
	devboard *remoteTi50.DUTControlAndreiboard
}

// DevBoard connects to devboardsvc server and returns the DevBoard instance.
func (v *Value) DevBoard(ctx context.Context, bufLen int, readTimeout time.Duration) (ti50.DevBoard, error) {
	if v.devboard != nil {
		if err := v.devboard.Close(ctx); err != nil {
			return nil, err
		}
	}
	v.devboard = remoteTi50.NewDUTControlAndreiboard(v.grpcConn, bufLen, readTimeout)
	return v.devboard, nil
}

type impl struct {
	imageValue *ImageValue
	hostPort   string
	v          *Value
}

func (i *impl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if s.ParentValue() != nil {
		i.imageValue = s.ParentValue().(*ImageValue)
	}

	i.hostPort = s.RequiredVar(DevBoardService)
	i.v = &Value{}

	if err := i.dialGrpc(ctx); err != nil {
		s.Fatal("dial grpc: ", err)
	}

	if err := i.flashImage(ctx); err != nil {
		s.Fatal("flash image: ", err)
	}

	return i.v
}

func (i *impl) Reset(ctx context.Context) error {
	if i.v.devboard != nil {
		if err := i.v.devboard.Reset(ctx); err != nil {
			return err
		}
		if err := i.v.devboard.Close(ctx); err != nil {
			return err
		}
		i.v.devboard = nil
	}
	return nil
}

func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	if i.v.devboard != nil {
		if err := i.v.devboard.Close(ctx); err != nil {
			s.Error("Failed to close devboard: ", err)
		}
		i.v.devboard = nil
	}
	if i.v.grpcConn != nil {
		if err := i.v.grpcConn.Close(); err != nil {
			s.Error("Failed to close grpc: ", err)
		}
		i.v.grpcConn = nil
	}
}

func (i *impl) String() string {
	if i.imageValue != nil {
		return DevBoardService + "_" + i.imageValue.ImageType()
	}
	return DevBoardService
}

// flashImage flashes the fixture's image onto the board.
func (i *impl) flashImage(ctx context.Context) error {
	if i.imageValue == nil {
		return nil
	}

	f := i.imageValue.ImagePath()
	if f == "" {
		return nil
	}

	board := remoteTi50.NewDUTControlAndreiboard(i.v.grpcConn, 0, 0*time.Second)
	defer board.Close(ctx)

	testing.ContextLog(ctx, "Flash image: ", f)
	if err := board.FlashImage(ctx, f); err != nil {
		return err
	}

	/*testing.ContextLog(ctx, "Initial board reset")
	if err := board.Reset(ctx); err != nil {
		return err
	}
	*/

	return nil
}

// dialGrpc connects to the devboardsvc host.
func (i *impl) dialGrpc(ctx context.Context) error {
	conn, err := grpc.DialContext(ctx, i.hostPort, grpc.WithInsecure())
	if err != nil {
		return err
	}
	i.v.grpcConn = conn
	return nil
}
