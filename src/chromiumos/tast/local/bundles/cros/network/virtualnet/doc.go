// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package virtualnet provides utilities for setting up an environment that
// contains a virtual interface and is able to run multiple services in a
// separate netns and chroot to provide different functionalities (e.g., DHCP
// and SLAAC) to this interface.
package virtualnet
