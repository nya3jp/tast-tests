// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uidetection provides image-based UI detections/interactions.
package uidetection

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"chromiumos/tast/errors"
	pb "chromiumos/tast/local/uidetection/api"
)

type uiDetector struct {
	keyType string
	key     string
	server  string
}

func (d *uiDetector) sendDetectionRequest(ctx context.Context, imagePng []byte, request *pb.DetectionRequest) (*pb.UiDetectionResponse, error) {
	// Create the UI detection request.
	uiDetectionRequest := &pb.UiDetectionRequest{
		ImagePng: imagePng,
		Request:  request,
	}

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(d.keyType, d.key))

	// Establish grpc connection to the server.
	conn, err := grpc.Dial(
		d.server,
		grpc.WithTransportCredentials(credentials.NewTLS(nil)),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to establish connection to ui detection server")
	}
	defer conn.Close()

	client := pb.NewUiDetectionServiceClient(conn)

	return client.ExecuteDetection(ctx, uiDetectionRequest)
}
