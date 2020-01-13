// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package commander provides common command line interface and utilities
// for network related components.
package commander

import "chromiumos/tast/host"

// Commander is an interface for those who provides Command() function.
// It is used to hold both dut.DUT and host.SSH object.
// TODO(crbug.com/1019537): use a more suitable ssh object.
type Commander interface {
	Command(string, ...string) *host.Cmd
}
