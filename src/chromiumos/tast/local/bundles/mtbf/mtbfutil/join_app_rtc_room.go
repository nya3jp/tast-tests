// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbfutil

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/mtbfutil/apprtc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/testing"
)

const (
	encodeHistName   = "Media.RTCVideoEncoderInitEncodeSuccess"
	decodeHistName   = "Media.RTCVideoDecoderInitDecodeSuccess"
	profileHistName  = "Media.RTCVideoEncoderProfile"
	encodeHistCount  = 1
	decodeHistCount  = 1
	profileHistCount = 11
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         JoinAppRtcRoom,
		Desc:         "Join AppRtc room",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Vars:         []string{"dynamic.var"},
	})
}

func getHistogramCount(ctx context.Context, cr *chrome.Chrome, name string) (*int, error) {
	v := 0
	h, err := metrics.GetHistogram(ctx, cr, name)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoNoHist, err, name)
	}

	if len((*h).Buckets) <= 0 {
		return nil, mtbferrors.New(mtbferrors.VideoZeroBucket, nil, name)
	}

	v = int((*h).Buckets[0].Count)
	return &v, nil
}

// JoinAppRtcRoom open notification center
func JoinAppRtcRoom(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	histogramURL := "chrome://histograms/Media.R"
	roomName, ok := s.Var("dynamic.var")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "dynamic.var"))
	}

	beforeJoinEncodeCnt, mtbferr := getHistogramCount(ctx, cr, encodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		s.Fatal(mtbferr)
	}

	beforeJoinProfileCnt, mtbferr := getHistogramCount(ctx, cr, profileHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		s.Fatal(mtbferr)
	}

	beforeJoinDecodeCnt, mtbferr := getHistogramCount(ctx, cr, decodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		s.Fatal(mtbferr)
	}

	s.Log("Open AppRtc Room")
	conn, mtbferr := apprtc.JoinRtcRoom(ctx, cr, roomName)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	testing.Sleep(ctx, time.Second*5)

	s.Logf("Open %s page and reload twice", histogramURL)
	connHist, mtbferr := mtbfchrome.NewConn(ctx, cr, histogramURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer connHist.Close()
	defer connHist.CloseTarget(ctx)

	for i := 0; i < 2; i++ {
		if err := connHist.EvalPromise(ctx, "window.location.reload()", nil); err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.ChromeExeJs, err, "window.location.reload()"))
		}
		testing.Sleep(ctx, time.Second*1)
	}

	afterJoinEncodeCnt, mtbferr := getHistogramCount(ctx, cr, encodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		s.Fatal(mtbferr)
	}

	afterJoinProfileCnt, mtbferr := getHistogramCount(ctx, cr, profileHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		s.Fatal(mtbferr)
	}

	afterJoinDecodeCnt, mtbferr := getHistogramCount(ctx, cr, decodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		s.Fatal(mtbferr)
	}

	if afterJoinEncodeCnt != nil && beforeJoinEncodeCnt == nil && afterJoinProfileCnt != nil && beforeJoinProfileCnt == nil {
		encodeDiff := *afterJoinEncodeCnt - 0
		profileDiff := *afterJoinProfileCnt - 0

		if encodeDiff != encodeHistCount {
			s.Error(mtbferrors.New(mtbferrors.VideoHistNotEqual, nil, encodeHistName, encodeHistCount, encodeDiff, 0, *afterJoinEncodeCnt))
		}

		if profileDiff != profileHistCount {
			s.Error(mtbferrors.New(mtbferrors.VideoHistNotEqual, nil, profileHistName, profileHistCount, profileDiff, 0, *afterJoinProfileCnt))
		}
	}

	if afterJoinDecodeCnt != nil && beforeJoinDecodeCnt == nil {
		decodeDiff := *afterJoinDecodeCnt - 0
		if decodeDiff != decodeHistCount {
			s.Error(mtbferrors.New(mtbferrors.VideoHistNotEqual, nil, decodeHistName, decodeHistCount, decodeDiff, 0, *afterJoinDecodeCnt))
		}
	}
}
