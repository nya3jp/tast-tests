// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package features

import "testing"

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

const model = "redrix"

const hdrnet = "hdrnet"
const fooFeature = "foo_feature"

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

func TestNewFromJSONInput(t *testing.T) {
	modelConf, err := NewFromJSONInput(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes")
	}

	if len(modelConf.FeatureSet) != 3 {
		t.Errorf("Expect to find 3 feature entries; found %v", len(modelConf.FeatureSet))
	}

	var featureConf map[string]interface{}
	if err := modelConf.FeatureConfig(hdrnet, &featureConf, []byte(fakeHDRnetConfig)); err != nil {
		t.Errorf("Failed to get feature config for %s: %s", hdrnet, err)
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

func TestIsFeatureEnabled(t *testing.T) {
	modelConf, err := NewFromJSONInput(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes")
	}

	if !modelConf.IsFeatureEnabled(hdrnet) {
		t.Errorf("Expect %s to be enabled; found otherwise", hdrnet)
	}

	if modelConf.IsFeatureEnabled(fooFeature) {
		t.Errorf("Expect %s to be not enabled; found otherwise", fooFeature)
	}
}

func TestFeatureConfigFilePath(t *testing.T) {
	modelConf, err := NewFromJSONInput(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes")
	}

	const expPath = "/etc/camera/hdrnet_config_redrix.json"
	if p, err := modelConf.FeatureConfigFilePath(hdrnet); p != expPath || err != nil {
		t.Errorf("Expect %s config path to be %v, found %v", hdrnet, expPath, p)
	}

	if p, err := modelConf.FeatureConfigFilePath(fooFeature); p != "" || err == nil {
		t.Errorf("Expect %s to have no config path, found otherwise", fooFeature)
	}
}

func TestFeatureConfig(t *testing.T) {
	modelConf, err := NewFromJSONInput(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes")
	}
	var featureConf map[string]interface{}
	if err := modelConf.FeatureConfig(hdrnet, &featureConf, []byte(fakeHDRnetConfig)); err != nil {
		t.Errorf("Failed to get feature config for %s: %s", hdrnet, err)
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

func TestOverrideFeatureConfig(t *testing.T) {
	modelConf, err := NewFromJSONInput(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes")
	}
	var featureConf map[string]interface{}
	if err := modelConf.FeatureConfig(hdrnet, &featureConf, []byte(fakeHDRnetConfig)); err != nil {
		t.Errorf("Failed to get feature config for %s: %s", hdrnet, err)
	}

	const enableControl = "hdrnet_enable"
	var enableValue = !(featureConf[enableControl] == true)
	const fooControl = "foo"
	const fooValue = "bar"
	newConf := map[string]interface{}{
		enableControl: enableValue,
		fooControl:    fooValue,
	}
	ret := UpdateFeatureConfig(featureConf, newConf)

	if ret[enableControl] != false {
		t.Errorf("Expect %s to be set to %v; found %v", enableControl, enableValue, ret[enableControl])
	}

	if v, ok := ret[fooControl]; !ok || v != fooValue {
		t.Errorf("Expect %s to be set to %v; found %v", fooControl, fooValue, v)
	}

	const hdrRatioControl = "hdr_ratio"
	var hdrRatioValue = featureConf[hdrRatioControl]
	if ret[hdrRatioControl] != hdrRatioValue {
		t.Errorf("Expect %s to be set to %v; found %v", hdrRatioControl, hdrRatioValue, ret[hdrRatioControl])
	}
}
