// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. boot_perf_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. perfetto_system_tracing_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. upstart_service.proto

// Package platform provides the BootPerfService, PerfettoSystemTracingService
// and UpstartService.
package platform

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/platform
