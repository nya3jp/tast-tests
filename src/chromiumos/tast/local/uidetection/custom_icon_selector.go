// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	pb "chromiumos/tast/local/uidetection/api"
	"context"
)

// CustomIconSelector is a selector for the custom icon.
type CustomIconSelector struct {
	*Selector
	Icon []byte
}

// CustomIcon returns a CustomIconSelector for a given icon.
func CustomIcon(icon []byte) *CustomIconSelector {
	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_CustomIconDetectionRequest{
			CustomIconDetectionRequest: &pb.CustomIconDetectionRequest{
				IconPng: icon,
			},
		},
	}
	return &CustomIconSelector{
		Selector: newFromRequest(detectionRequest),
		Icon:     icon,
	}

}

func (s *CustomIconSelector) find(ctx context.Context, d *uiDetector, imagePng []byte) (*Location, error) {
	response, err := d.sendDetectionRequest(ctx, s.request, imagePng)
	if err != nil {
		return nil, err
	}

	// TODO return error if there are multiple matches.
	iconBoundingBoxes := response.GetBoundingBoxes()

	result := &Location{}

	if len(iconBoundingBoxes) < 1 {
		return nil, errors.New("failed to find matching element with word")
	}

	boundingBox := iconBoundingBoxes[0]
	result.TopLeft = coords.NewPoint(int(boundingBox.GetLeft()), int(boundingBox.GetTop()))
	result.BottomRight = coords.NewPoint(int(boundingBox.GetRight()), int(boundingBox.GetBottom()))
	return result, nil
}
