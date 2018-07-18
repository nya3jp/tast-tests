// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package deqp contains information needed for running DEQP tests.
package deqp

// Tests contains the names of the DEQP tests to run. Some may be skipped
// depending on the supported graphics APIs. This list is directly obtained from
// autotest/files/client/site_tests/graphics_dEQP/master/bvt.txt.
var Tests = [...]string{
	"dEQP-GLES2.info.vendor",
	"dEQP-GLES2.info.renderer",
	"dEQP-GLES2.info.version",
	"dEQP-GLES2.info.shading_language_version",
	"dEQP-GLES2.info.extensions",
	"dEQP-GLES2.info.render_target",
	"dEQP-GLES2.functional.prerequisite.state_reset",
	"dEQP-GLES2.functional.prerequisite.clear_color",
	"dEQP-GLES2.functional.prerequisite.read_pixels",
	"dEQP-GLES3.info.vendor",
	"dEQP-GLES3.info.renderer",
	"dEQP-GLES3.info.version",
	"dEQP-GLES3.info.shading_language_version",
	"dEQP-GLES3.info.extensions",
	"dEQP-GLES3.info.render_target",
	"dEQP-GLES3.functional.prerequisite.state_reset",
	"dEQP-GLES3.functional.prerequisite.clear_color",
	"dEQP-GLES3.functional.prerequisite.read_pixels",
	"dEQP-GLES31.info.vendor",
	"dEQP-GLES31.info.renderer",
	"dEQP-GLES31.info.version",
	"dEQP-GLES31.info.shading_language_version",
	"dEQP-GLES31.info.extensions",
	"dEQP-GLES31.info.render_target",
	"dEQP-VK.info.build",
	"dEQP-VK.info.device",
	"dEQP-VK.info.platform",
	"dEQP-VK.info.memory_limits",
	"dEQP-VK.api.smoke.create_sampler",
	"dEQP-VK.api.smoke.create_shader",
	"dEQP-VK.api.info.instance.physical_devices",
	"dEQP-VK.api.info.instance.layers",
	"dEQP-VK.api.info.instance.extensions",
	"dEQP-VK.api.info.device.features",
	"dEQP-VK.api.info.device.queue_family_properties",
	"dEQP-VK.api.info.device.memory_properties",
	"dEQP-VK.api.info.device.layers",
}
