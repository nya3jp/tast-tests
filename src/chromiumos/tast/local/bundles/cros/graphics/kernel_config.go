// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/kernel"
	"chromiumos/tast/testing"
)

var (
	// should be builtin i.e. MODULE = y
	isBuiltin = []string{}
	// should be enabled i.e. MODULE = y or MODULE = m
	//  should be disabled.
	isDissabled = []string{
		"DRM_KMS_FB_HELPER",
		"FB",
		"FB_CFB_COPYAREA",
		"FB_CFB_FILLRECT",
		"FB_CFB_IMAGEBLIT",
		"FB_CFB_REV_PIXELS_IN_BYTE",
		"FB_SIMPLE",
		"FB_SYS_COPYAREA",
		"FB_SYS_FOPS",
		"FB_SYS_FILLRECT",
		"FB_SYS_IMAGEBLIT",
		"FB_VIRTUAL",
	}
	isEnabled = []string{}
	// should be a module i.e. MODULE = m
	isModule = []string{}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelConfig,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Examine a kernel build CONFIG list to verify related flags",
		// TODO(syedfaaiz): Add to CQ once it is green and stable.
		Attr: []string{"group:graphics", "graphics_nightly"},
		Contacts: []string{"syedfaaiz@google.com",
			"chromeos-gfx@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
		Timeout:      5 * time.Minute,
	})
}

func KernelConfig(ctx context.Context, s *testing.State) {
	kernelConfigMap, err := kernel.ReadKernelConfig(ctx)
	if err != nil {
		s.Fatal("an error occured : ", err)
	}
	// check if any builtin config is not configured
	missingBuiltinModules := make([]string, 0)
	for _, configKey := range isBuiltin {
		if kernelConfigMap[configKey] != "y" {
			missingBuiltinModules = append(missingBuiltinModules, configKey)
		}
	}
	if len(missingBuiltinModules) > 0 {
		s.Fatal("Error, kernel is missing the following configuration(s): ", missingBuiltinModules)
	}
	// check if any unwanted config is enabled.
	unwantedConfig := make([]string, 0)
	for _, configKey := range isDissabled {
		if _, exists := kernelConfigMap[configKey]; exists && kernelConfigMap[configKey] != "n" {
			s.Log(kernelConfigMap[configKey])
			unwantedConfig = append(unwantedConfig, configKey)
		}
	}
	if len(unwantedConfig) > 0 {
		s.Fatal("Error, kernel should not have the following configuration(s): ", unwantedConfig)
	}
	// check if any config is not enabled
	unEnabledModules := make([]string, 0)
	for _, configKey := range isEnabled {
		if kernelConfigMap[configKey] != "y" && kernelConfigMap[configKey] != "m" {
			unEnabledModules = append(unEnabledModules, configKey)
		}
	}
	if len(unEnabledModules) > 0 {
		s.Fatal("Error, kernel should have the following configuration(s) enabled: ", unEnabledModules)
	}
	// check if any module is not enabled
	missingModules := make([]string, 0)
	for _, configKey := range isModule {
		if kernelConfigMap[configKey] != "m" {
			missingModules = append(missingModules, configKey)
		}
	}
	if len(missingModules) > 0 {
		s.Fatal("Error, kernel is missing the following module(s): ", missingModules)
	}

	return
}
