// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package features provides utilities for loading and parsing on-device feature
// config files.
package features

import (
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/errors"
)

// ModelConfig contains all the metadata for features enabled on a device model.
type ModelConfig struct {
	FeatureSet []FeatureMetadata `json:"feature_set"`
}

// FeatureMetadata mainly points to the model-specific feature config file for a
// feature.
type FeatureMetadata struct {
	Type           string `json:"type"`
	ConfigFilePath string `json:"config_file_path"`
}

// New returns a ModelConfig for the given |model| by loading and parsing the
// on-device feature profile config if |jsonInput| is nil, or by parsing
// |jsonInput| directly if it's given.
func New(model string, jsonInput []byte) (*ModelConfig, error) {
	if jsonInput == nil {
		const featureProfilePath = "/etc/camera/feature_profile.json"
		var err error
		if jsonInput, err = ioutil.ReadFile(featureProfilePath); err != nil {
			return nil, errors.Wrap(err, "cannot load feature profile config")
		}
	}

	var featureProfile map[string]ModelConfig
	if err := json.Unmarshal(jsonInput, &featureProfile); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal feature profile config")
	}

	conf, ok := featureProfile[model]
	if ok != true {
		return nil, errors.Errorf("feature set config for model %s doesn't exist", model)
	}

	return &conf, nil
}

// IsFeatureEnabled returns if a feature is enabled in the feature profile
// config. A feature is enabled if there is a corresponding FeatureMetadata
// entry in |modelConf.FeatureSet|. Note that the feature itself may be set to
// run-time disabled through in its config file for testing or debugging
// purposes.
func IsFeatureEnabled(modelConf *ModelConfig, ftype string) bool {
	for _, m := range modelConf.FeatureSet {
		if m.Type == ftype {
			return true
		}
	}
	return false
}

// GetFeatureConfigFilePath gets the config file path of feature |ftype|.
func GetFeatureConfigFilePath(modelConf *ModelConfig, ftype string) (string, error) {
	for _, m := range modelConf.FeatureSet {
		if m.Type == ftype {
			return m.ConfigFilePath, nil
		}
	}
	return "", errors.Errorf("feature config for type %s doesn't exist", ftype)
}

// GetFeatureConfig returns the unmarshaled JSON object in |featureConf|
// containing the feature config of |ftype|. The feature config is loaded from
// the on-device file from metadata in |modelConf| if |jsonInput| is nil, or
// parsed from |jsonInput| directly if it's given.
func GetFeatureConfig(modelConf *ModelConfig, ftype string, featureConf interface{}, jsonInput []byte) error {
	for _, m := range modelConf.FeatureSet {
		if m.Type == ftype {
			if jsonInput == nil {
				var err error
				jsonInput, err = ioutil.ReadFile(m.ConfigFilePath)
				if err != nil {
					return errors.Wrapf(err, "cannot load feature config file %s", m.ConfigFilePath)
				}
			}
			if err := json.Unmarshal(jsonInput, featureConf); err != nil {
				return errors.Wrap(err, "cannot unmarshal feature config")
			}
			return nil
		}
	}
	return errors.Errorf("feature config for type %s doesn't exist", ftype)
}

// SetOverrideFeatureConfig marshals and writes the feature config |conf| into
// the file on |filePath|.
func SetOverrideFeatureConfig(conf interface{}, filePath string) error {
	bytes, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return errors.Wrap(err, "cannot marshal override config")
	}

	if err := ioutil.WriteFile(filePath, bytes, 0666); err != nil {
		return errors.Wrapf(err, "cannot write override config file %s", filePath)
	}

	return nil
}
