// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. touch_screen_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. touch_pad_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. keyboard_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. mouse_service.proto

// Package inputs provides all inputs related types compiled from protobuf.
package inputs

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/inputs
