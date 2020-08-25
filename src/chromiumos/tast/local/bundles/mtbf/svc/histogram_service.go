// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package svc

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			svc.RegisterHistogramServiceServer(srv, &HistogramService{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// HistogramService implements tast.mtbf.svc.HistogramService.
type HistogramService struct {
	// embedded structure to get the implementation of LoginReusePre
	chrome.SvcLoginReusePre
}

// GetFirstBucket returns the first bucket with the requested name
func (s *HistogramService) GetFirstBucket(ctx context.Context, in *svc.GetFirstBucketRequest) (*svc.GetFirstBucketResponse, error) {

	testing.ContextLog(ctx, "CommService - GetFirstBucket called")

	err := s.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	tconn, err := s.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	histogramURL := "chrome://histograms/" + in.Name
	histogramConn, err := s.CR.NewConn(ctx, histogramURL)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenInURL, err, histogramURL)
	}
	defer histogramConn.Close()
	defer histogramConn.CloseTarget(ctx)

	histogram, err := metrics.GetHistogram(ctx, tconn, in.Name)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoGetHist, err, in.Name)
	}
	var rsp svc.GetFirstBucketResponse
	if len(histogram.Buckets) != 0 {
		bucket := histogram.Buckets[0]
		rsp.Min = bucket.Min
		rsp.Max = bucket.Max
		rsp.Count = bucket.Count
	}

	return &rsp, nil
}
