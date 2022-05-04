// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package features provides utilities for loading and parsing on-device feature
// config files.
package features

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

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
// on-device feature profile config.
func New(model string) (*ModelConfig, error) {
	const featureProfilePath = "/etc/camera/feature_profile.json"
	jsonInput, err := ioutil.ReadFile(featureProfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load feature profile config")
	}
	return NewFromJSONInput(model, jsonInput)
}

// NewFromJSONInput returns a ModelConfig for the given |model| in the parsed
// feature profile config from |jsonInput|.
func NewFromJSONInput(model string, jsonInput []byte) (*ModelConfig, error) {
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
func (c *ModelConfig) IsFeatureEnabled(ftype string) bool {
	for _, m := range c.FeatureSet {
		if m.Type == ftype {
			return true
		}
	}
	return false
}

// FeatureConfigFilePath gets the config file path of feature |ftype|.
func (c *ModelConfig) FeatureConfigFilePath(ftype string) (string, error) {
	for _, m := range c.FeatureSet {
		if m.Type == ftype {
			return m.ConfigFilePath, nil
		}
	}
	return "", errors.Errorf("feature config for type %s doesn't exist", ftype)
}

// FeatureConfig returns the unmarshaled JSON object in |featureConf|
// containing the feature config of |ftype|. The feature config is loaded from
// the on-device file from metadata in |modelConf| if |jsonInput| is nil, or
// parsed from |jsonInput| directly if it's given.
func (c *ModelConfig) FeatureConfig(ftype string, featureConf interface{}, jsonInput []byte) error {
	for _, m := range c.FeatureSet {
		if m.Type != ftype {
			continue
		}
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
	return errors.Errorf("feature config for type %s doesn't exist", ftype)
}

// UpdateFeatureConfig takes |origConf| and, add new settings or overwriting
// existing settings with the key value pairs in |newConf|, and return the
// resulting config.
func UpdateFeatureConfig(origConf, newConf map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range origConf {
		result[k] = v
	}
	for k, v := range newConf {
		result[k] = v
	}
	return result
}

// WriteFeatureConfig marshals and writes the feature config |conf| into
// the file on |filePath|. If |overwrite| is true, then the file is cleared and
// overwritten with the configs in |conf|; otherwise |conf| is used to extend or
// overwrite the existing config in the file.
func WriteFeatureConfig(conf map[string]interface{}, filePath string, overwrite bool) error {
	var loadExistingConfig = func(file string) (map[string]interface{}, error) {
		if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		// Load the existing settings in the file.
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read existing feature config from file %q", file)
		}
		c := make(map[string]interface{})
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal existing feature config from file %q", file)
		}
		return c, nil
	}

	var writeConfig = func(file string) error {
		f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to open feature config file %q", file)
		}
		defer f.Close()

		var output = conf
		if !overwrite {
			// Load the existing feature config in the file.
			c, err := loadExistingConfig(file)
			if err != nil {
				return errors.Wrap(err, "failed to load existing feature config")
			}
			if c == nil {
				// The file may not exist.
				c = make(map[string]interface{})
			}
			output = UpdateFeatureConfig(c, conf)
		}

		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "failed to marshal feature config %s", output)
		}
		length, err := f.Write(data)
		if err != nil {
			return errors.Wrapf(err, "failed to write feature config to %q", file)
		}
		f.Truncate(int64(length))
		log.Printf("Save device feature config to file: %q", file)
		return nil
	}

	if err := writeConfig(filePath); err != nil {
		return err
	}
	return nil
}
