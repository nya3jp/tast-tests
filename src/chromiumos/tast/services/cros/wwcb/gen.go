// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. display_service.proto

// Package wwcb provides all wwcb related types compiled from protobuf.
package wwcb

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/wwcb
