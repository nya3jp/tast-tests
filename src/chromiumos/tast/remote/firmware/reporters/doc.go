// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package reporters is a collection of canonical methods to obtain test-relevant information about a DUT, in the form of <information>, error.
// A non-nil error indicates that the information was not succesfuflly obtained.
// It should not modify the DUT state.
package reporters
