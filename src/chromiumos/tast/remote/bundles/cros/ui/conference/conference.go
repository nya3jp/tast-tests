// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

const (
	// TwoRoomSize creates a conference room with 2 participants
	TwoRoomSize = 2
	// SmallRoomSize creates a conference room with 5 participants
	SmallRoomSize = 5
	// LargeRoomSize creates a conference room with 17 participants
	LargeRoomSize = 17
	// ClassRoomSize creates a conference room with 38 participants
	ClassRoomSize = 38
)

// TestParameters defines the test parameters for conference.
type TestParameters struct {
	// Size is the conf room size.
	Size int
	// ScreenMode defines the screen mode: tablet or clamshell.
	ScreenMode string
	// Tier defines the test tier: basic, plus, or premium.
	Tier string
}

// Client provides a interface for different conference type, includes GoogleMeet, Zoom.
type Client interface {
	MeetScenario(ctx context.Context, in *pb.MeetScenarioRequest, opts ...grpc.CallOption) (*empty.Empty, error)
}

// Run starts a gRPC call to run the specified user scenario.
func Run(ctx context.Context, s *testing.State, client Client, tier string, roomSize int, tabletMode, extendedDisplay bool) {
	account := s.RequiredVar("ui.cuj_username")
	password := s.RequiredVar("ui.cuj_password")

	if _, err := client.MeetScenario(ctx, &pb.MeetScenarioRequest{
		Account:         account,
		Password:        password,
		Tier:            tier,
		RoomSize:        int64(roomSize),
		TabletMode:      tabletMode,
		ExtendedDisplay: extendedDisplay,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
