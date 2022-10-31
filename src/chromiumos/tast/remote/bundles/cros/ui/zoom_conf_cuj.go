// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Host a Zoom video conference and do presentation to participants",
		Contacts:     []string{"jane.yang@cienet.com", "cienet-development@googlegroups.com"},
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
				Timeout: 15*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.TwoRoomSize,
				},
			}, {
				Name:              "basic_lacros_two",
				Timeout:           15*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.TwoRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_small",
				Timeout: 15*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.SmallRoomSize,
				},
			}, {
				Name:              "basic_lacros_small",
				Timeout:           15*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.SmallRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_large",
				Timeout: 20*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "basic_lacros_large",
				Timeout:           20*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_class",
				Timeout: 20*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "basic_lacros_class",
				Timeout:           20*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Basic,
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_large",
				Timeout: 20*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "plus_lacros_large",
				Timeout:           20*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_class",
				Timeout: 20*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "plus_lacros_class",
				Timeout:           20*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Plus,
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "premium_large",
				Timeout: 20*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     conference.Premium,
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "premium_lacros_large",
				Timeout:           20*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     conference.Premium,
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
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
	if _, err := client.RunZoomScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            int64(param.Tier),
		RoomType:        int64(param.RoomType),
		ExtendedDisplay: false,
		CameraVideoPath: remoteCameraVideoPath,
		IsLacros:        param.IsLacros,
	}); err != nil {
		s.Fatal("Failed to run Zoom Scenario: ", err)
	}
}
