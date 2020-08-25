// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multimedia

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/video/youtube"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			multimedia.RegisterYoutubeServiceServer(srv, &YoutubeService{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

type YoutubeService struct {
	chrome.SvcLoginReusePre
}

func (s *YoutubeService) IsPlaying(ctx context.Context, req *multimedia.IsPlayingRequest) (*multimedia.IsPlayingResponse, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, err
	}

	conn, err := s.CR.NewConnForTarget(ctx, chrome.MatchTargetURL(req.Url))
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeExistTarget, err, req.Url)
	}

	defer conn.Close()
	if err := youtube.IsPlaying(ctx, conn, 5*time.Second); err != nil {
		return &multimedia.IsPlayingResponse{IsPlaying: false}, err
	}

	return &multimedia.IsPlayingResponse{IsPlaying: true}, nil
}

func (s *YoutubeService) PlayYoutubeVideo(ctx context.Context, req *multimedia.PlayYoutubeVideoRequest) (*empty.Empty, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, err
	}

	conn, err := mtbfchrome.NewConn(ctx, s.CR, req.URL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	testing.ContextLog(ctx, "Document is ready")
	youtube.PlayVideo(ctx, conn)

	return &empty.Empty{}, nil
}
