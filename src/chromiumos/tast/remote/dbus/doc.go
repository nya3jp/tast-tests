// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dbus includes remote dbus utilities. Should only include things that
// make sense as a remote call, such as monitoring. Anything that can otherwise
// be in a local dbus package should be there, not here.
package dbus
