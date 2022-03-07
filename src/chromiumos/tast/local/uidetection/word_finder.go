// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	pb "google.golang.org/genproto/googleapis/chromeos/uidetection/v1"
)

// Word returns a finder for a given word.
func Word(word string) *Finder {
	detectionRequest := &pb.DetectionRequest{
		DetectionRequestType: &pb.DetectionRequest_WordDetectionRequest{
			WordDetectionRequest: &pb.WordDetectionRequest{
				Word: word,
			},
		},
	}
	return newFromRequest(detectionRequest, word)
}
