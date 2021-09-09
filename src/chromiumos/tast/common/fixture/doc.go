// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture holds names of the fixtures available in Tast.
// Defining fixture name constants in this package makes it easier to search
// fixture implementations and their usages in tests.
// Because local tests/fixtures can depend on remote fixtures but local
// packages are prohibited to import remote packages these constants are
// defined in common/, instead of local/ and remote/.
package fixture
