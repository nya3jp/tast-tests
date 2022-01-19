// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package support identifies the different common features routers may support
// through interfaces that each router may or may not support.
//
// You can use with WithX functions (e.g. WithCapture), where X is a support
// interface to easily handle validating whether a given router has that support
// and/or to cast a Router router to a specific support interface to use that
// interface's specific functionality.
package support
