// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"fmt"
	"path/filepath"

	pb "google.golang.org/genproto/googleapis/chromeos/uidetection/v1"
)

// CustomIcon returns a finder for a given icon.
func CustomIcon(iconFile string, paramList ...CustomIconParam) *Finder {
	// Read icon image from file.
	icon, err := readImage(iconFile)
	if err != nil {
		panic(fmt.Sprintf("failed to read the icon: %q", iconFile))
	}

	customIconParams := DefaultCustomIconParams()
	for _, param := range paramList {
		param(customIconParams)
	}

	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_CustomIconDetectionRequest{
			CustomIconDetectionRequest: &pb.CustomIconDetectionRequest{
				IconPng:                icon,
				MatchCount:             customIconParams.MatchCount,
				MinConfidenceThreshold: customIconParams.MinConfidence,
			},
		},
	}
	return newFromRequest(detectionRequest, filepath.Base(iconFile))
}
