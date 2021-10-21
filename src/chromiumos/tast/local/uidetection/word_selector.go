// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	pb "chromiumos/tast/local/uidetection/api"
	"context"

	"google.golang.org/protobuf/proto"
)

// WordSelector is a selector for the word.
type WordSelector struct {
	*Selector
	Word string
}

// TextBlock returns a WordSelector for a given word.
func Word(word string) *WordSelector {
	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_WordDetectionRequest{
			WordDetectionRequest: &pb.WordDetectionRequest{
				Word: proto.String(word),
			},
		},
	}

	return &WordSelector{
		Selector: newFromRequest(detectionRequest),
		Word:     word,
	}
}

func (s *WordSelector) find(ctx context.Context, d *uiDetector, imagePng []byte) (*Location, error) {
	response, err := d.sendDetectionRequest(ctx, s.request, imagePng)
	if err != nil {
		return nil, err
	}

	// TODO return error if there are multiple matches.
	wordBoundingBoxes := response.GetBoundingBoxes()

	result := &Location{}
	for _, boundingBox := range wordBoundingBoxes {
		if boundingBox.GetText() == s.Word {
			result.TopLeft = coords.NewPoint(int(boundingBox.GetLeft()), int(boundingBox.GetTop()))
			result.BottomRight = coords.NewPoint(int(boundingBox.GetRight()), int(boundingBox.GetBottom()))
			return result, nil
		}
	}

	return nil, errors.New("failed to find matching element with word")
}
