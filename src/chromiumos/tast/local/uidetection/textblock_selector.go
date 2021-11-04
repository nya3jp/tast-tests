// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"strings"

	pb "chromiumos/tast/local/uidetection/api"
)

// TextBlock returns a selector for a given textblock.
func TextBlock(words []string) *Selector {
	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_TextBlockDetectionRequest{
			TextBlockDetectionRequest: &pb.TextBlockDetectionRequest{
				Words: words,
			},
		},
	}
	return newFromRequest(detectionRequest, strings.Join(words, ","))
}
