// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"strings"

	pb "google.golang.org/genproto/googleapis/chromeos/uidetection/v1"
)

// TextBlock returns a finder for a given textblock.
// Use TextBlock if you want to find a group of nearby text,
// generally representing an entire text element.
// E.g. use uidetection.TextBlock([]string{"Save", "As"})
// to find the "Save As" menu item.
func TextBlock(words []string, paramList ...TextParam) *Finder {
	textParams := DefaultTextParams()
	for _, param := range paramList {
		param(textParams)
	}

	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_TextBlockDetectionRequest{
			TextBlockDetectionRequest: &pb.TextBlockDetectionRequest{
				Words:              words,
				RegexMode:          textParams.RegexMode,
				DisableApproxMatch: textParams.DisableApproxMatch,
				MaxEditDistance:    &textParams.MaxEditDistance,
			},
		},
	}
	return newFromRequest(detectionRequest, strings.Join(words, ","))
}
