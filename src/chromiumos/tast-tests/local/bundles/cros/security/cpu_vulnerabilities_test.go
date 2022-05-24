// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast-tests/local/bundles/cros/security/

// See src/chromiumos/tast-tests/local/crostini/params.go for more documentation

import (
	"testing"

	"chromiumos/tast-tests/common/genparams"
	"chromiumos/tast-tests/local/crostini"
)

func TestSecurityParams(t *testing.T) {
	params := crostini.MakeTestParams(t)
	genparams.Ensure(t, "cpu_vulnerabilities_crostini.go", params)
}
