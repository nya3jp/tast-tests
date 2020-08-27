// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package genparams

import (
	"bytes"
	"strconv"
	"text/template"
)

var funcs = map[string]interface{}{
	"quote": strconv.Quote,
}

// Template is a utility function to render a Go template into a string in a
// single call.
//
// Go's standard template engine is used to render a template string. See
// https://godoc.org/text/template for the template syntax.
//
// This function also installs a few helper function that can be called inside
// templates:
//
//  quote(s string) string - Quotes a string as a Go string literal.
func Template(t TestingT, text string, data interface{}) string {
	t.Helper()

	tmpl, err := template.New("").Funcs(funcs).Parse(text)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}
	return buf.String()
}
