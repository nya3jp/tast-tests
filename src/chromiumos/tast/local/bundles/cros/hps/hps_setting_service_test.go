// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"fmt"
	"testing"
	"time"

	durationpb "google.golang.org/protobuf/types/known/durationpb"

	pb "chromiumos/tast/services/cros/hps"
)

const (
	valid1 = `
	2022-05-04T04:10:50.189648Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=1m quick_dim=1m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:04.597625Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=2m quick_dim=2m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:04.608987Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=3m quick_dim=3m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=no-op use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:05.542450Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=4m quick_dim=4m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=shutdown use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:11.852847Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=5m quick_dim=5m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:13.780787Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=6m quick_dim=6m screen_off=7m30s lock=0s quick_lock=2m6s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:15.608926Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=8m quick_dim=7m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:46.090911Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=7m quick_dim=10s screen_off=7m30s lock=0s quick_lock=2m10s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=
	2022-05-10T04:12:46.090911Z INFO powerd: [state_controller.cc(1166)] Updated settings
	`
	valid2 = `
	2022-05-04T04:12:46.835388Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=1m quick_dim=1m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks= \n
	2022-05-06T00:30:20.734450Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=2m quick_dim=2m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-06T00:30:20.374935Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=3m quick_dim=3m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-06T00:30:20.734450Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=4m quick_dim=4m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-06T00:30:45.736758Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=6m quick_dim=5m screen_off=7m30s lock=0s quick_lock=2m10s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-09T01:48:07.360070Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=5m quick_dim=6s screen_off=5m30s lock=0s quick_lock=2m6s idle_warn=0s idle=6m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=
	2022-05-10T04:12:46.090911Z INFO powerd: [state_controller.cc(1166)] Updated settings
	`
	invalid1 = ""
	invalid2 = `2022-05-04T04:12:13.780787Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=7m quick_dim=6s screen_off=7m30s lock=0s quick_lock=2m6s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:15.608926Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=7m quick_dim=2m screen_off=7m30s lock=0s quick_lock=8m30s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	2022-05-04T04:12:46.090911Z INFO powerd: [state_controller.cc(1166)] Updated settings: dim=m quick_dim=10 screen_off=3 lock=0s quick_lock=2m10s idle_warn=0s idle==-=sdf30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=\n
	`
	invalid3 = `invalid\n
	very much invalid case`
)

func formatString(res *pb.RetrieveDimMetricsResponse) string {
	return fmt.Sprintf("DimDelay: %ds, ScreenOffDelay: %ds, LockDelay: %ds",
		res.DimDelay.Seconds, res.ScreenOffDelay.Seconds, res.LockDelay.Seconds)
}

func compareResult(result, expected *pb.RetrieveDimMetricsResponse) bool {
	return formatString(result) == formatString(expected)
}

type inputReq struct {
	content    []byte
	isQuickDim bool
}

func TestPowerdLastUpdatedSettings(t *testing.T) {
	for i, tc := range []struct {
		input  []byte
		output string
	}{
		{
			input:  []byte(valid1),
			output: "Updated settings: dim=7m quick_dim=10s screen_off=7m30s lock=0s quick_lock=2m10s idle_warn=0s idle=8m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=",
		},
		{
			input:  []byte(valid2),
			output: "Updated settings: dim=5m quick_dim=6s screen_off=5m30s lock=0s quick_lock=2m6s idle_warn=0s idle=6m30s (no-op) lid_closed=suspend use_audio=1 use_video=1 wake_locks=",
		},
	} {
		out := powerdLastUpdatedSettings(tc.input)
		if out != tc.output {
			t.Errorf(`Incorrect Output: index: %v
Expected %q
Got      %q`, i, tc.output, out)
		}
	}
}

func TestRetrieveDimMetricsValid(t *testing.T) {
	for i, tc := range []struct {
		input  *inputReq
		output *pb.RetrieveDimMetricsResponse
	}{
		{
			input: &inputReq{
				content:    []byte(valid1),
				isQuickDim: false,
			},
			output: &pb.RetrieveDimMetricsResponse{
				DimDelay:       durationpb.New(time.Duration(7*60) * time.Second),
				ScreenOffDelay: durationpb.New(time.Duration(0.5*60) * time.Second),
				LockDelay:      durationpb.New(time.Duration(60) * time.Second),
			},
		},
		{
			input: &inputReq{
				content:    []byte(valid1),
				isQuickDim: true,
			},
			output: &pb.RetrieveDimMetricsResponse{
				DimDelay:       durationpb.New(time.Duration(10) * time.Second),
				ScreenOffDelay: durationpb.New(time.Duration(120) * time.Second),
				LockDelay:      durationpb.New(time.Duration(0) * time.Second),
			},
		},
		{
			input: &inputReq{
				content:    []byte(valid2),
				isQuickDim: true,
			},
			output: &pb.RetrieveDimMetricsResponse{
				DimDelay:       durationpb.New(time.Duration(6) * time.Second),
				ScreenOffDelay: durationpb.New(time.Duration(120) * time.Second),
				LockDelay:      durationpb.New(time.Duration(0) * time.Second),
			},
		},
		{
			input: &inputReq{
				content:    []byte(valid2),
				isQuickDim: false,
			},
			output: &pb.RetrieveDimMetricsResponse{
				DimDelay:       durationpb.New(time.Duration(5*60) * time.Second),
				ScreenOffDelay: durationpb.New(time.Duration(30) * time.Second),
				LockDelay:      durationpb.New(time.Duration(60) * time.Second),
			},
		},
	} {
		result, err := processContent(tc.input.content, tc.input.isQuickDim)
		if err != nil {
			t.Errorf("error retrieving metrics: %q", err)
		}
		if !compareResult(result, tc.output) {
			t.Errorf(`Incorrect Output: index: %v, isQuickDim: %t
Expected %q
Got      %q`, i, tc.input.isQuickDim, formatString(tc.output), formatString(result))
		}
	}
}

func TestRetrieveDimMetricsInvalid(t *testing.T) {
	for _, tc := range []struct {
		input  *inputReq
		output *pb.RetrieveDimMetricsResponse
	}{
		{
			input: &inputReq{
				content:    []byte(invalid1),
				isQuickDim: false,
			},
			output: nil,
		},
		{
			input: &inputReq{
				content:    []byte(invalid1),
				isQuickDim: true,
			},
			output: nil,
		},
		{
			input: &inputReq{
				content:    []byte(invalid2),
				isQuickDim: false,
			},
			output: nil,
		},
		{
			input: &inputReq{
				content:    []byte(invalid2),
				isQuickDim: true,
			},
			output: nil,
		},
		{
			input: &inputReq{
				content:    []byte(invalid3),
				isQuickDim: false,
			},
			output: nil,
		},
		{
			input: &inputReq{
				content:    []byte(invalid3),
				isQuickDim: true,
			},
			output: nil,
		},
	} {
		result, err := processContent(tc.input.content, tc.input.isQuickDim)
		if err == nil {
			t.Errorf("didn't generate error for invalid case")
		}
		if result != tc.output {
			t.Errorf("Incorrect Output: expected %q, get %q", formatString(tc.output), formatString(result))
		}
	}
}
