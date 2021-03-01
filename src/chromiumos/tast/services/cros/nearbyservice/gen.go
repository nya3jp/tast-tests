// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . --go_out=plugins=grpc:../../../../.. nearby_share_service.proto

// Package nearbyservice provides the NearbyShareService.
package nearbyservice

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/nearbyservice
