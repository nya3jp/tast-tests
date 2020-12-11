// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/vm

import (
	"fmt"
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestFio(t *testing.T) {
	type paramData struct {
		Name string
		Kind string
		Job  string
	}

	jobs := []string{"boot", "login", "surfing", "randread", "randwrite", "seqread", "seqwrite", "stress_rw"}
	kind := []string{"block", "virtiofs", "p9"}

	var params []paramData
	for _, job := range jobs {
		for _, kind := range kind {
			params = append(params, paramData{
				Name: fmt.Sprintf("%s_%s", kind, job),
				Kind: kind,
				Job:  fmt.Sprintf("fio_%s.job", job),
			})
		}
	}

	code := genparams.Template(t,
		`{{ range . }}{
			Name: {{ .Name | fmt }},
			ExtraData: []string{ {{ .Job | fmt }} },
			Val: param{
				kind: {{ .Kind | fmt }},
				job: {{ .Job | fmt }},
			},
		},
		{{ end }}`,
		params)

	genparams.Ensure(t, "fio.go", code)
}
