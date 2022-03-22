// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	pb "google.golang.org/genproto/googleapis/chromeos/uidetection/v1"
)

// Word returns a finder for a given word.
func Word(word string, paramList ...TextParam) *Finder {
	textParams := DefaultTextParams()
	for _, param := range paramList {
		param(textParams)
	}
	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_WordDetectionRequest{
			WordDetectionRequest: &pb.WordDetectionRequest{
				Word:               word,
				RegexMode:          textParams.RegexMode,
				DisableApproxMatch: textParams.DisableApproxMatch,
				MaxEditDistance:    &textParams.MaxEditDistance,
			},
		},
	}
	return newFromRequest(detectionRequest, word)
}
