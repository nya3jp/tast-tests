// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package svc

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/mafredri/cdp/protocol/target"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/local/mtbf/mtbfutil/apprtc"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			svc.RegisterWebServiceServer(srv, &WebService{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// WebService implements tast.mtbf.svc.WebService.
type WebService struct {
	// embedded structure to get the implementation of LoginReusePre
	chrome.SvcLoginReusePre
}

// OpenURL open url
func (s *WebService) OpenURL(ctx context.Context, req *svc.OpenURLRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "WebService - OpenURL called")

	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	conn, err := mtbfchrome.NewConn(ctx, s.CR, req.Url)
	if err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeOpenInURL, err, req.Url)
	}

	if mtbferr := dom.WaitForDocumentReady(ctx, conn); mtbferr != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.VideoDocLoad, nil, req.Url)
	}

	return &empty.Empty{}, nil
}

// CloseURL close url
func (s *WebService) CloseURL(ctx context.Context, req *svc.CloseURLRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "WebService - CloseURL called")

	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	matcher := func(t *target.Info) bool {
		return strings.Contains(t.URL, req.Url)
	}
	conn, err := s.CR.NewConnForTarget(ctx, matcher)
	if err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeExistTarget, err, req.Url)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	return &empty.Empty{}, nil
}

func (s *WebService) Click(ctx context.Context, req *svc.ClickRequest) (*empty.Empty, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	conn, err := s.CR.NewConnForTarget(ctx, chrome.MatchTargetURL(req.Url))
	if err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeExistTarget, err, req.Url)
	}

	conn.EvalPromise(ctx, req.Selector+".click()", nil)

	return nil, nil
}

func (s *WebService) ClickLinkByName(ctx context.Context, req *svc.ClickLinkByNameRequest) (*empty.Empty, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	conn, err := s.CR.NewConnForTarget(ctx, chrome.MatchTargetURL(req.Url))
	if err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeExistTarget, err, req.Url)
	}

	findLinkJs := fmt.Sprintf(`
		[].filter.call(document.querySelectorAll("a"), a => a.textContent === %q)[0].click()
	`, req.Name)

	conn.EvalPromise(ctx, findLinkJs, nil)

	return nil, nil
}

// PlayElement play element
func (s *WebService) PlayElement(ctx context.Context, req *svc.PlayElementRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "WebService - PlayElement called")

	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	conn, err := s.CR.NewConnForTarget(ctx, chrome.MatchTargetURL(req.Url))
	if err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeExistTarget, err, req.Url)
	}
	if err := dom.PlayElement(ctx, conn, req.Selector); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.VideoPlayingEle, nil, req.Url)
	}
	return &empty.Empty{}, nil
}

// IsGmailChatRoomExists returns whether is Gmail chat room exists
func (s *WebService) IsGmailChatRoomExists(ctx context.Context, req *empty.Empty) (*svc.IsGmailChatRoomExistsResponse, error) {
	testing.ContextLog(ctx, "WebService - IsGmailChatRoomExists called")
	initSvc := &svc.IsGmailChatRoomExistsResponse{IsExists: false}

	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return initSvc, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return initSvc, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	var chatRoomCount int
	gmailURL := "https://mail.google.com/mail/u/0/#inbox"
	gmailConn, err := s.CR.NewConnForTarget(ctx, chrome.MatchTargetURL(gmailURL))
	if err != nil {
		return initSvc, mtbferrors.New(mtbferrors.ChromeExistTarget, err, gmailURL)
	}

	script := "document.querySelector('%s').childElementCount"
	chatRoomParent := "body > div.dw > div > div > div > div.no > div:nth-child(3)"
	query := fmt.Sprintf(script, chatRoomParent)
	gmailConn.Eval(ctx, query, &chatRoomCount)

	return &svc.IsGmailChatRoomExistsResponse{IsExists: (chatRoomCount != 0)}, nil
}

// JoinAppRTCRoom joins an AppRTC room.
func (s *WebService) JoinAppRTCRoom(ctx context.Context, req *svc.JoinAppRTCRoomRequest) (*empty.Empty, error) {
	const (
		histogramURL = "chrome://histograms/Media.R"

		encodeHistName  = "Media.RTCVideoEncoderInitEncodeSuccess"
		decodeHistName  = "Media.RTCVideoDecoderInitDecodeSuccess"
		profileHistName = "Media.RTCVideoEncoderProfile"

		encodeHistCount  = 1
		decodeHistCount  = 1
		profileHistCount = 11
	)

	testing.ContextLog(ctx, "WebService - JoinAppRTCRoom called")

	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	tconn, err := s.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	beforeJoinEncodeCnt, mtbferr := getHistogramCount(ctx, tconn, encodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		return nil, mtbferr
	}

	beforeJoinProfileCnt, mtbferr := getHistogramCount(ctx, tconn, profileHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		return nil, mtbferr
	}

	beforeJoinDecodeCnt, mtbferr := getHistogramCount(ctx, tconn, decodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		return nil, mtbferr
	}

	testing.ContextLog(ctx, "Open AppRtc Room")
	if mtbferr := apprtc.JoinRtcRoom(ctx, s.CR, req.RoomName); mtbferr != nil {
		return nil, mtbferr
	}

	testing.Sleep(ctx, time.Second*5)

	testing.ContextLogf(ctx, "Open %s page and reload twice", histogramURL)
	connHist, mtbferr := mtbfchrome.NewConn(ctx, s.CR, histogramURL)
	if mtbferr != nil {
		return nil, mtbferr
	}
	defer connHist.Close()
	defer connHist.CloseTarget(ctx)

	for i := 0; i < 2; i++ {
		if err := connHist.EvalPromise(ctx, "window.location.reload()", nil); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeExeJs, err, "window.location.reload()")
		}
		testing.Sleep(ctx, time.Second*1)
	}

	afterJoinEncodeCnt, mtbferr := getHistogramCount(ctx, tconn, encodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		return nil, mtbferr
	}

	afterJoinProfileCnt, mtbferr := getHistogramCount(ctx, tconn, profileHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		return nil, mtbferr
	}

	afterJoinDecodeCnt, mtbferr := getHistogramCount(ctx, tconn, decodeHistName)
	if mtbferr != nil && !strings.Contains(mtbferr.Error(), "has no value") {
		return nil, mtbferr
	}

	if afterJoinEncodeCnt != nil && beforeJoinEncodeCnt == nil && afterJoinProfileCnt != nil && beforeJoinProfileCnt == nil {
		encodeDiff := *afterJoinEncodeCnt - 0
		profileDiff := *afterJoinProfileCnt - 0

		if encodeDiff != encodeHistCount {
			return nil, mtbferrors.New(mtbferrors.VideoHistNotEqual, nil, encodeHistName, encodeHistCount, encodeDiff, 0, *afterJoinEncodeCnt)
		}

		if profileDiff != profileHistCount {
			return nil, mtbferrors.New(mtbferrors.VideoHistNotEqual, nil, profileHistName, profileHistCount, profileDiff, 0, *afterJoinProfileCnt)
		}
	}

	if afterJoinDecodeCnt != nil && beforeJoinDecodeCnt == nil {
		decodeDiff := *afterJoinDecodeCnt - 0
		if decodeDiff != decodeHistCount {
			return nil, mtbferrors.New(mtbferrors.VideoHistNotEqual, nil, decodeHistName, decodeHistCount, decodeDiff, 0, *afterJoinDecodeCnt)
		}
	}

	return &empty.Empty{}, nil
}

func getHistogramCount(ctx context.Context, tconn *chrome.TestConn, name string) (*int, error) {
	v := 0

	h, err := metrics.GetHistogram(ctx, tconn, name)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoNoHist, err, name)
	}

	if len((*h).Buckets) <= 0 {
		return nil, mtbferrors.New(mtbferrors.VideoZeroBucket, nil, name)
	}

	v = int((*h).Buckets[0].Count)
	return &v, nil
}
