// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/cros/ui/setup"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMeetCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Host a Google Meet video conference and do presentation to participants",
		Contacts:     []string{"jane.yang@cienet.com", "xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		ServiceDeps: []string{
			"tast.cros.ui.ConferenceService",
		},
		Data: []string{conference.CameraVideo, conference.TraceConfigFile},
		Vars: []string{
			"ui.use_real_camera",
			"ui.collectTrace", // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Params: []testing.Param{
			{
				Name:    "basic_two",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.TwoRoomSize,
				},
			}, {
				Name:              "basic_lacros_two",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.TwoRoomSize,
					IsLacros: true,
				},
			}, {
				Name:              "basic_two_crosbolt",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.TwoRoomSize,
				},
			}, {
				Name:    "basic_small",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.SmallRoomSize,
				},
			}, {
				Name:              "basic_lacros_small",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.SmallRoomSize,
					IsLacros: true,
				},
			}, {
				Name:              "basic_small_crosbolt",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.SmallRoomSize,
				},
			}, {
				Name:    "basic_large",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "basic_lacros_large",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:              "basic_large_crosbolt",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:    "basic_class",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "basic_lacros_class",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:              "basic_class_crosbolt",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:    "plus_large",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "plus_lacros_large",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_class",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "plus_lacros_class",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "premium_large",
				Timeout: time.Minute*50 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Premium,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "premium_lacros_large",
				Timeout:           time.Minute*50 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Premium,
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_no_meet",
				Timeout: time.Minute*10 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.NoRoom,
				},
			}, {
				Name:              "plus_lacros_no_meet",
				Timeout:           time.Minute*10 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.NoRoom,
					IsLacros: true,
				},
			}, {
				Name:    "premium_no_meet",
				Timeout: time.Minute*10 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Premium,
					RoomType: conference.NoRoom,
				},
			}, {
				Name:              "premium_lacros_no_meet",
				Timeout:           time.Minute*10 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Premium,
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
	if val, ok := s.Var("ui.use_real_camera"); ok {
		useRealCamera, err = strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert ui.use_real_camera var to bool: ", err)
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

	if collect, ok := s.Var("ui.collectTrace"); ok && collect == "enable" {
		remoteTraceConfigFilePath, err := conference.PushFileToTmpDir(ctx, s, dut, conference.TraceConfigFile)
		if err != nil {
			s.Fatal("Failed to push file to DUT's tmp directory: ", err)
		}
		defer dut.Conn().CommandContext(ctx, "rm", remoteTraceConfigFilePath).Run()
	}

	client := pb.NewConferenceServiceClient(c.Conn)
	if _, err := client.RunGoogleMeetScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            int64(param.Tier),
		RoomType:        int64(param.RoomType),
		ExtendedDisplay: false,
		CameraVideoPath: remoteCameraVideoPath,
		IsLacros:        param.IsLacros,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
