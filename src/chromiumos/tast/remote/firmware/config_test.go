// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"testing"

	"chromiumos/tast/errors"
)

var mockData = map[string][]byte{
	"DEFAULTS": []byte(`{
		"platform": "DEFAULTS",
		"parent": null,
		"firmware_screen": 1
	}`),
	"myplatform": []byte(`{
		"platform": "myplatform",
		"firmware_screen": 2
	}`),
}

func newMockConfigFactory() *ConfigFactory {
	bytesFromMockData := func(platform string) ([]byte, error) {
		b, ok := mockData[platform]
		if !ok {
			return nil, errors.Errorf("platform %s not found in mockData", platform)
		}
		return b, nil
	}
	return &ConfigFactory{loadBytes: bytesFromMockData}
}

func TestNewConfig(t *testing.T) {
	cf := newMockConfigFactory()
	cfg, err := cf.NewConfig("myplatform")
	if err != nil {
		t.Error(err, "creating config for myplatform")
	}
	if cfg.Platform != "myplatform" {
		t.Errorf("cfg has Platform %q, want 'my_platform'", cfg.Platform)
	}
	if cfg.FirmwareScreen != 2 {
		t.Errorf("cfg has FirmwareScreen %d, want 2", cfg.FirmwareScreen)
	}
}
