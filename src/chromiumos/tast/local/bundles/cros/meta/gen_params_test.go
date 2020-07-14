// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"fmt"
	"strings"
	"testing"

	"chromiumos/tast/common/generate"
)

func TestGenParamsParams(t *testing.T) {
	var params []string
	for _, i := range []int{1, 2, 3} {
		p := fmt.Sprintf(`{
	Name: "%[1]d",
	Val: %[1]d,
},
`, i)
		params = append(params, p)
	}
	generate.Params(t, "gen_params.go", strings.Join(params, ""))
}
