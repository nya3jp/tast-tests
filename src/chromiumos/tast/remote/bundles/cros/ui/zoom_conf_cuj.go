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
				Timeout: time.Minute*15 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.TwoRoomSize,
				},
			}, {
				Name:              "basic_lacros_two",
				Timeout:           time.Minute*15 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.TwoRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_small",
				Timeout: time.Minute*15 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.SmallRoomSize,
				},
			}, {
				Name:              "basic_lacros_small",
				Timeout:           time.Minute*15 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.SmallRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_large",
				Timeout: time.Minute*20 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "basic_lacros_large",
				Timeout:           time.Minute*20 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "basic_class",
				Timeout: time.Minute*20 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "basic_lacros_class",
				Timeout:           time.Minute*20 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "basic",
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_large",
				Timeout: time.Minute*20 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "plus_lacros_large",
				Timeout:           time.Minute*20 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "plus_class",
				Timeout: time.Minute*20 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.ClassRoomSize,
				},
			}, {
				Name:              "plus_lacros_class",
				Timeout:           time.Minute*20 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "plus",
					RoomType: conference.ClassRoomSize,
					IsLacros: true,
				},
			}, {
				Name:    "premium_large",
				Timeout: time.Minute*20 + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					Tier:     "premium",
					RoomType: conference.LargeRoomSize,
				},
			}, {
				Name:              "premium_lacros_large",
				Timeout:           time.Minute*20 + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					Tier:     "premium",
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
		Tier:            param.Tier,
		RoomType:        int64(param.RoomType),
		ExtendedDisplay: false,
		CameraVideoPath: remoteCameraVideoPath,
		IsLacros:        param.IsLacros,
	}); err != nil {
		s.Fatal("Failed to run Zoom Scenario: ", err)
	}
}
