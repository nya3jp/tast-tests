// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate protoc -I . -I ../../../../../../../../platform2/system_api/dbus/attestation --go_out=plugins=grpc:../../../../.. attestation_dbus_service.proto webauthn_service.proto
//go:generate protoc -I . -I ../../../../../../../../platform2/system_api/dbus/attestation --go_out=plugins=grpc:../../../../.. attestation_dbus_service.proto ownership.proto

package hwsec

// Run the following command in CrOS chroot to regenerate protocol buffer bindings:
//
// ~/trunk/src/platform/tast/tools/go.sh generate chromiumos/tast/services/cros/hwsec
