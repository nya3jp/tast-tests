// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. audio_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. automation_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. check_power_menu_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. check_virtual_keyboard_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. chrome_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. chrome_ui_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. screen_recorder_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. screenlock_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. tconn_service.proto

// Package ui provides all ui related types compiled from protobuf.
package ui

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/ui
