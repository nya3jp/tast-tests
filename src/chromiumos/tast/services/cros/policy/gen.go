// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. policy.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. system_timezone.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. client_certificate_service.proto
//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. device_minimum_version_service.proto

// Package policy provides the PolicyService
package policy

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/policy
