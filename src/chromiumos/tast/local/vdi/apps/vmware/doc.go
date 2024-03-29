// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vmware holds the implementation of the /vdi/apps/vdiconnector.VDIInt
// interface for Vmware application. It allows tests to retrieve VDI connector
// by calling s.FixtValue().(fixtures.HasVdiConnector).VdiConnector().
// Ultimately, this lets tests that execute VDI CUJ to be parameterized as
// long as fixture has functions that are defined by
// vdi/fixtures.HasVdiConnector attached to its return type.
package vmware
