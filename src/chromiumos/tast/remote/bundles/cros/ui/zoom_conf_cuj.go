// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ZoomConfCUJ,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Host a Zoom video conference and do presentation to participants",
		Contacts:     []string{"jane.yang@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		ServiceDeps: []string{
			"tast.cros.ui.ConferenceService",
		},
		Data: []string{conference.CameraVideo},
		Params: []testing.Param{
			{
				Name:    "basic_two",
				Timeout: time.Minute * 15,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.TwoRoomSize,
				},
			}, {
				Name:    "basic_small",
				Timeout: time.Minute * 15,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.SmallRoomSize,
				},
			}, {
				Name:    "basic_large",
				Timeout: time.Minute * 20,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.LargeRoomSize,
				},
			}, {
				Name:    "basic_class",
				Timeout: time.Minute * 20,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.ClassRoomSize,
				},
			}, {
				Name:    "plus_large",
				Timeout: time.Minute * 20,
				Val: conference.TestParameters{
					Tier: "plus",
					Size: conference.LargeRoomSize,
				},
			}, {
				Name:    "plus_class",
				Timeout: time.Minute * 20,
				Val: conference.TestParameters{
					Tier: "plus",
					Size: conference.ClassRoomSize,
				},
			}, {
				Name:    "premium_large",
				Timeout: time.Minute * 20,
				Val: conference.TestParameters{
					Tier: "premium",
					Size: conference.LargeRoomSize,
				},
			},
		},
	})
}

func ZoomConfCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	// Shorten ctx to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	dut := s.DUT()
	c, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer c.Close(cleanupCtx)

	remoteCameraVideoPath, err := conference.PushFileToTmpDir(ctx, s, dut, conference.CameraVideo)
	if err != nil {
		s.Fatal("Failed to push file to DUT's tmp directory: ", err)
	}
	defer dut.Conn().CommandContext(ctx, "rm", remoteCameraVideoPath).Run()

	client := pb.NewConferenceServiceClient(c.Conn)
	if _, err := client.RunZoomScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            param.Tier,
		RoomSize:        int64(param.Size),
		ExtendedDisplay: false,
		CameraVideoPath: remoteCameraVideoPath,
	}); err != nil {
		s.Fatal("Failed to run Zoom Scenario: ", err)
	}
}
