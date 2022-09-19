// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/spera/conference"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/spera"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMeetCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Host a Google Meet video conference and do presentation to participants",
		Contacts:     []string{"jane.yang@cienet.com", "xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		ServiceDeps: []string{
			"tast.cros.spera.ConferenceService2",
		},
		Data: []string{conference.CameraVideo, conference.TraceConfigFile},
		Vars: []string{
			"spera.use_real_camera",
			"spera.collectTrace", // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Params: []testing.Param{
			{
				Name:    "basic_two",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.TwoRoomSize,
				},
			}, {
				Name:              "basic_lacros_two",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.TwoRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_small",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.SmallRoomSize,
				},
			}, {
				Name:              "basic_lacros_small",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.SmallRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_large",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "basic_lacros_large",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_class",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "basic_lacros_class",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_large",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "plus_lacros_large",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_class",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "plus_lacros_class",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "premium_large",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "premium",
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "premium_lacros_large",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "premium",
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_no_meet",
				Timeout: time.Minute*10 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.NoRoom,
				},
			}, {
				Name:              "plus_lacros_no_meet",
				Timeout:           time.Minute*10 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.NoRoom,
					IsLacros: true,
				},
			}, {
				Name:    "premium_no_meet",
				Timeout: time.Minute*10 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "premium",
					RoomType: conference.NoRoom,
				},
			}, {
				Name:              "premium_lacros_no_meet",
				Timeout:           time.Minute*10 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "premium",
					RoomType: conference.NoRoom,
					IsLacros: true,
				},
			},
		},
	})
}

func GoogleMeetCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	dut := s.DUT()
	c, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer c.Close(ctx)
	var remoteCameraVideoPath string
	var useRealCamera bool // Default is false.
	if val, ok := s.Var("spera.use_real_camera"); ok {
		useRealCamera, err = strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert spera.use_real_camera var to bool: ", err)
		}
	}
	// Use fake camera by default.
	if !useRealCamera {
		remoteCameraVideoPath, err = conference.PushFileToTmpDir(ctx, s, dut, conference.CameraVideo)
		if err != nil {
			s.Fatal("Failed to push file to DUT's tmp directory: ", err)
		}
		defer dut.Conn().CommandContext(ctx, "rm", remoteCameraVideoPath).Run()
	}

	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		remoteTraceConfigFilePath, err := conference.PushFileToTmpDir(ctx, s, dut, conference.TraceConfigFile)
		if err != nil {
			s.Fatal("Failed to push file to DUT's tmp directory: ", err)
		}
		defer dut.Conn().CommandContext(ctx, "rm", remoteTraceConfigFilePath).Run()
	}
	client := pb.NewConferenceService2Client(c.Conn)
	if _, err := client.RunGoogleMeetScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            param.Tier,
		RoomType:        int64(param.RoomType),
		ExtendedDisplay: false,
		CameraVideoPath: remoteCameraVideoPath,
		IsLacros:        param.IsLacros,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
