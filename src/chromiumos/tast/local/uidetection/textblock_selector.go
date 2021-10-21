// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	pb "chromiumos/tast/local/uidetection/api"
	"context"
	"strings"
)

// TextBlockSelector is a selector for the textblock.
type TextBlockSelector struct {
	*Selector
	Words []string
}

// TextBlock returns a TextBlockSelector for a given textblock.
func TextBlock(words []string) *TextBlockSelector {
	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_TextBlockDetectionRequest{
			TextBlockDetectionRequest: &pb.TextBlockDetectionRequest{
				Words: words,
			},
		},
	}
	return &TextBlockSelector{
		Selector: newFromRequest(detectionRequest),
		Words:    words,
	}
}

func (s *TextBlockSelector) find(ctx context.Context, d *uiDetector, imagePng []byte) (*Location, error) {
	response, err := d.sendDetectionRequest(ctx, s.request, imagePng)
	if err != nil {
		return nil, err
	}

	textBlockBoundingBoxes := response.GetBoundingBoxes()

	result := &Location{}
	// TODO return error if there are multiple matches.

	for _, boundingBox := range textBlockBoundingBoxes {
		if Equals(strings.Split(boundingBox.GetText(), " "), s.Words) {
			result.TopLeft = coords.NewPoint(int(boundingBox.GetLeft()), int(boundingBox.GetTop()))
			result.BottomRight = coords.NewPoint(int(boundingBox.GetRight()), int(boundingBox.GetBottom()))
			return result, nil
		}
	}

	return nil, errors.New("failed to find matching element with word")
}
