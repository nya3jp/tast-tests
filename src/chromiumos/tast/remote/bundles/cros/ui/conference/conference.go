// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

const (
	// GoogleMeet uses Google Meet for CUJ conference scenario testing.
	GoogleMeet = "googlemeet"
	// Zoom uses Zoom for CUJ conference scenario testing.
	Zoom = "zoom"
)

const (
	// OneRoomSize creates a conference room with 2 participants
	OneRoomSize = 2
	// SmallRoomSize creates a conference room with 5 participants
	SmallRoomSize = 5
	// MediumRoomSize creates a conference room with 17 participants
	MediumRoomSize = 17
	// LargeRoomSize creates a conference room with 38 participants
	LargeRoomSize = 38
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

type account struct {
	Name     string
	Password string
}

type conferenceClient interface {
	MeetScenario(ctx context.Context, in *pb.MeetScenarioRequest, opts ...grpc.CallOption) (*empty.Empty, error)
}

// Run starts a gRPC call to run the specified user scenario.
func Run(ctx context.Context, s *testing.State, conferenceName, tier string, roomSize int, tabletMode bool) {
	dut := s.DUT()
	u1, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer u1.Close(ctx)

	var client conferenceClient
	switch conferenceName {
	case GoogleMeet:
		client = pb.NewGoogleMeetServiceClient(u1.Conn)
	case Zoom:
		client = pb.NewZoomServiceClient(u1.Conn)
	default:
		s.Fatal("Unrecognize conference type: ", conferenceName)
	}
	clientAccount := account{Name: s.RequiredVar("ui.cuj_username"), Password: s.RequiredVar("ui.cuj_password")}

	if _, err := client.MeetScenario(ctx, &pb.MeetScenarioRequest{
		Account:    clientAccount.Name,
		Password:   clientAccount.Password,
		Tier:       tier,
		RoomSize:   int64(roomSize),
		TabletMode: tabletMode,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
