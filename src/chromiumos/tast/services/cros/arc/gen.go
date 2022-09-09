// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. suspend_service.proto
//go:generate protoc -I . -I ../../../common/perf/perfpb --go_out=plugins=grpc:../../../../.. perf_boot_service.proto
//go:generate protoc -I . -I ../../../common/perf/perfpb --go_out=plugins=grpc:../../../../.. power_perf_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. gmscore_cache_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. ureadahead_pack_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. adb_over_usb_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. tts_cache_service.proto

package arc

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/arc
