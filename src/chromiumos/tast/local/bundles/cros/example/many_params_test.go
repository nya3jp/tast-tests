// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"fmt"
	"testing"

	"chromiumos/tast/common/genparams"
)

func TestManyParams(t *testing.T) {
	type paramData struct {
		Name         string
		SoftwareDeps []string
		Options      []string
		Expr         string
	}

	var params []paramData
	for _, arc := range []struct {
		name  string
		value bool
	}{
		{"noarc", false},
		{"arc", true},
	} {
		for _, expr := range []struct {
			name  string
			value string
		}{
			{"url", "location.href"},
			{"state", "document.readyState"},
		} {
			p := paramData{
				Name:         fmt.Sprintf("%s_%s", arc.name, expr.name),
				SoftwareDeps: []string{"chrome"},
				Expr:         expr.value,
			}
			if arc.value {
				p.SoftwareDeps = append(p.SoftwareDeps, "android")
				p.Options = append(p.Options, "chrome.ARCEnabled()")
			}
			params = append(params, p)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
	Name: {{ .Name | fmt }},
	ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
	Val: manyParamsParams{
		{{ if .Options }}
		Options: []chrome.Option{
			{{ range .Options }}
			{{ . }},
			{{ end }}
		},
		{{ end }}
		Expr: {{ .Expr | fmt }},
	},
},
{{ end }}`, params)
	genparams.Ensure(t, "many_params.go", code)
}
