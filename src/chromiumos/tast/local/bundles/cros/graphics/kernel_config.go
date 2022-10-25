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
	isEnabled = []string{}
	//  should be disabled.
	isDisabled = []string{
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
		SoftwareDeps: []string{},
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
	for _, configKey := range isBuiltin {
		if _, exists := kernelConfigMap[configKey]; exists && kernelConfigMap[configKey] != "y" {
			s.Error("Error, kernel is missing the following configuration: ", configKey)
		}
	}
	// check if any unwanted config is enabled.
	for _, configKey := range isDisabled {
		if _, exists := kernelConfigMap[configKey]; exists && kernelConfigMap[configKey] != "n" {
			s.Error("Error, kernel should not have the following configuration: ", configKey)
		}
	}
	// check if any config is not enabled
	for _, configKey := range isEnabled {
		if _, exists := kernelConfigMap[configKey]; exists && kernelConfigMap[configKey] != "y" &&
			kernelConfigMap[configKey] != "m" {
			s.Error("Error, kernel should have the following configuration enabled:", configKey)
		}
	}
	// check if any module is not enabled
	for _, configKey := range isModule {
		if _, exists := kernelConfigMap[configKey]; exists && kernelConfigMap[configKey] != "m" {
			s.Error("Error, kernel is missing the following module: ", configKey)
		}
	}
	return
}
