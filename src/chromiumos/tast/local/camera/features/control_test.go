// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package features

import "testing"

const (
	fakeFeatureProfile = `{
  "kano": {
    "feature_set": [ {
      "type": "hdrnet",
      "config_file_path": "/etc/camera/hdrnet_config_kano.json"
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
      "config_file_path": "/etc/camera/hdrnet_config_redrix4es.json"
    }, {
      "type": "gcam_ae",
      "config_file_path": "/etc/camera/gcam_ae_config_redrix4es.json"
    } ]
  }
}`
	model            = "redrix"
	hdrnet           = "hdrnet"
	fooFeature       = "foo_feature"
	fakeHDRnetConfig = `{
  "dump_buffer": false,
  "hdr_ratio": 3,
  "hdrnet_enable": true,
  "iir_filter_strength": 0.75,
  "log_frame_metadata": false,
  "max_gain_blend_threshold": 0,
  "range_filter_sigma": 0,
  "spatial_filter_sigma": 0.5
}`
)

func TestNewModelConfigFromJSON(t *testing.T) {
	modelConf, err := NewModelConfigFromJSON(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes: %v", err)
	}

	if len(modelConf.FeatureSet) != 3 {
		t.Errorf("Unexpected feature entries: got %d, want %d", len(modelConf.FeatureSet), 3)
	}
}

func TestIsFeatureEnabled(t *testing.T) {
	modelConf, err := NewModelConfigFromJSON(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes: %v", err)
	}

	for _, tst := range []struct {
		f        string
		expected bool
	}{
		{hdrnet, true},
		{fooFeature, false},
	} {
		v := modelConf.IsFeatureEnabled(tst.f)
		if v != tst.expected {
			t.Errorf("Unexpected %q enable state: got %v, want %v", tst.f, v, tst.expected)
		}
	}
}

func TestFeatureConfigFilePath(t *testing.T) {
	modelConf, err := NewModelConfigFromJSON(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes: %v", err)
	}

	const expPath = "/etc/camera/hdrnet_config_redrix.json"
	if p, err := modelConf.FeatureConfigFilePath(hdrnet); err != nil {
		t.Errorf("Failed to get %q config file path: %v", hdrnet, err)
	} else if p != expPath {
		t.Errorf("Unexpected %q config file path: got %v, want %v", hdrnet, p, expPath)
	}

	if p, err := modelConf.FeatureConfigFilePath(fooFeature); err == nil {
		t.Errorf("Failed to get %q config file path: %v", fooFeature, err)
	} else if p != "" {
		t.Errorf("Expect %q to have no config file path, found otherwise", fooFeature)
	}
}

func TestFeatureConfig(t *testing.T) {
	modelConf, err := NewModelConfigFromJSON(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes: %v", err)
	}
	featureConf := NewFeatureConfig()
	if err := modelConf.FeatureConfig(hdrnet, featureConf, []byte(fakeHDRnetConfig)); err != nil {
		t.Errorf("Failed to get feature config for %s: %s", hdrnet, err)
	}
	for k, v := range map[string]interface{}{
		"dump_buffer":         false,
		"hdrnet_enable":       true,
		"iir_filter_strength": 0.75,
	} {
		if featureConf[k] != v {
			t.Errorf("Unexpected %s: got %v, want %v", k, featureConf[k], v)
		}
	}
}

func TestMeldFeatureConfig(t *testing.T) {
	modelConf, err := NewModelConfigFromJSON(model, []byte(fakeFeatureProfile))
	if err != nil {
		t.Fatalf("Failed to parse input JSON bytes: %v", err)
	}
	featureConf := NewFeatureConfig()
	if err := modelConf.FeatureConfig(hdrnet, featureConf, []byte(fakeHDRnetConfig)); err != nil {
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
	ret := MeldFeatureConfig(featureConf, newConf)

	if ret[enableControl] != enableValue {
		t.Errorf("Unexpected %s: got %v, want %v", enableControl, ret[enableControl], enableValue)
	}

	if v, ok := ret[fooControl]; !ok {
		t.Errorf("Expect %s to be set, found otherwise", fooControl)
	} else if v != fooValue {
		t.Errorf("Unexpected %s: got %v, want %v", fooControl, v, fooValue)
	}

	const hdrRatioControl = "hdr_ratio"
	var hdrRatioValue = featureConf[hdrRatioControl]
	if ret[hdrRatioControl] != hdrRatioValue {
		t.Errorf("Unexpected %s: got %v, want %v", hdrRatioControl, ret[hdrRatioControl], hdrRatioValue)
	}
}
