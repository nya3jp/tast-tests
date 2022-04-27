// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package features

import "testing"

func TestFeatureConfigParsing(t *testing.T) {
	const fakeFeatureProfile = `{
  "kano": {
    "feature_set": [ {
      "type": "hdrnet",
      "config_file_path": "/etc/camera/hdrnet_config_kano.json"
    }, {
      "type": "gcam_ae",
      "config_file_path": "/etc/camera/gcam_ae_config_kano.json"
    }, {
      "type": "face_detection",
      "config_file_path": "/etc/camera/face_detection_config_kano.json"
    } ]
  },
  "redrix": {
    "feature_set": [ {
      "type": "hdrnet",
      "config_file_path": "/etc/camera/hdrnet_config_redrix.json"
    }, {
      "type": "gcam_ae",
      "config_file_path": "/etc/camera/gcam_ae_config_redrix.json"
    }, {
      "type": "face_detection",
      "config_file_path": "/etc/camera/face_detection_config_redrix.json"
    } ]
  },
  "redrix4es": {
    "feature_set": [ {
      "type": "hdrnet",
      "config_file_path": "/etc/camera/hdrnet_config_redrix.json"
    }, {
      "type": "gcam_ae",
      "config_file_path": "/etc/camera/gcam_ae_config_redrix.json"
    }, {
      "type": "face_detection",
      "config_file_path": "/etc/camera/face_detection_config_redrix.json"
    } ]
  }
}`

	modelConf, err := New("redrix", []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes")
	}

	if len(modelConf.FeatureSet) != 3 {
		t.Errorf("Expect to find 3 feature entries; found %v", len(modelConf.FeatureSet))
	}

	const ftype = "hdrnet"
	const fakeHDRnetConfig = `{
  "dump_buffer": false,
  "hdr_ratio": 3,
  "hdrnet_enable": true,
  "iir_filter_strength": 0.75,
  "log_frame_metadata": false,
  "max_gain_blend_threshold": 0,
  "range_filter_sigma": 0,
  "spatial_filter_sigma": 0.5
}`

	var featureConf map[string]interface{}
	if err := GetFeatureConfig(modelConf, "hdrnet", &featureConf, []byte(fakeHDRnetConfig)); err != nil {
		t.Errorf("Failed to get feature config for %s: %s", ftype, err)
	}
	for k, v := range map[string]interface{}{
		"dump_buffer":         false,
		"hdrnet_enable":       true,
		"iir_filter_strength": 0.75,
	} {
		if featureConf[k] != v {
			t.Errorf("Expect %s == %v; found %v", k, v, featureConf[k])
		}
	}
}
