// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"fmt"

	pb "chromiumos/tast/local/uidetection/api"
)

// CustomIcon returns a selector for a given icon.
func CustomIcon(iconFile string) *Selector {
	// Read icon image from file.
	icon, err := ReadImage(iconFile)
	if err != nil {
		panic(fmt.Sprintf("failed to read the icon: %q", iconFile))
	}

	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_CustomIconDetectionRequest{
			CustomIconDetectionRequest: &pb.CustomIconDetectionRequest{
				IconPng: icon,
			},
		},
	}
	return newFromRequest(detectionRequest, iconFile)
}
