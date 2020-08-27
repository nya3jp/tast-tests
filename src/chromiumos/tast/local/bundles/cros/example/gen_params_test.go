// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestGenParamsParams(t *testing.T) {
	const tmpl = `{{ range . }}{
	Name: {{ quote .Name }},
	Val: {{ .Value }},
},
{{ end }}`
	params := []struct {
		Name  string
		Value int
	}{
		{"a", 1},
		{"b", 2},
	}
	genparams.Ensure(t, "gen_params.go", genparams.Template(t, tmpl, params))
}
