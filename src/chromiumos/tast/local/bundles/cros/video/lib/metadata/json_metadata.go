// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package metadata parses json metadata files.
package metadata

import (
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/errors"
)

// jsonDecodeMetadata stores parsed metadata from test video json file.
type jsonDecodeMetadata struct {
	Profile            string   `json:"profile"`
	Width              int      `json:"width"`
	Height             int      `json:"height"`
	FrameRate          int      `json:"frame_rate"`
	NumFrames          int      `json:"num_frames"`
	NumFragments       int      `json:"num_fragments"`
	MD5Checksums       []string `json:"md5_checksums"`
	ThumbnailChecksums []string `json:"thumbnail_checksums"`
}

// GetDecodeMetadataFromJSONFile returns parsed decode metadata from input jsonFile.
func GetDecodeMetadataFromJSONFile(jsonFile string) (jsonDecodeMetadata, error) {
	var decodeMetadata jsonDecodeMetadata

	b, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return decodeMetadata, errors.Wrapf(err, "failed to read json file %s", jsonFile)
	}

	if err := json.Unmarshal(b, &decodeMetadata); err != nil {
		return decodeMetadata, errors.Wrap(err, "failed to unmarshal decode metadata")
	}

	return decodeMetadata, nil
}
