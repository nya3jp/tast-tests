// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/ast/astutil"

	"chromiumos/tast/caller"
)

const updateEnv = "TAST_GENERATE_UPDATE"

func Params(t *testing.T, file, params string) {
	oldCode, err := ioutil.ReadFile(file)
	if err != nil {
		t.Logf("generate.Params: %v; skipping", err)
		return
	}

	fs := token.NewFileSet()
	root, err := parser.ParseFile(fs, file, oldCode, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	// Determine the import alias of chromiumos/tast/testing.
	var testingAlias string
	for _, im := range root.Imports {
		path, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			continue
		}
		if path == "chromiumos/tast/testing" {
			if im.Name == nil {
				testingAlias = "testing"
			} else {
				testingAlias = im.Name.Name
			}
		}
	}
	if testingAlias == "" {
		t.Fatal("generate.Params: chromiumos/tast/testing is not imported")
	}

	paramsLit := fmt.Sprintf("[]%s.Param{\n%s\n}", testingAlias, params)
	paramsExpr, err := parser.ParseExpr(paramsLit)
	if err != nil {
		t.Fatalf("generate.Params: %v", err)
	}

	// Replace Params in testing.Test literals.
	hits := 0
	astutil.Apply(root, func(cur *astutil.Cursor) bool {
		comp, ok := cur.Node().(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := comp.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Test" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != testingAlias {
			return true
		}

		elts := append([]ast.Expr(nil), comp.Elts...)
		for i, elt := range elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if id, ok := kv.Key.(*ast.Ident); !ok || id.Name != "Params" {
				continue
			}
			elts = append(elts[:i], elts[i+1:]...)
			break
		}

		elts = append(elts, &ast.KeyValueExpr{
			Key:   &ast.Ident{Name: "Params"},
			Value: paramsExpr,
		})

		repl := &ast.CompositeLit{
			Type: &ast.SelectorExpr{
				X:   &ast.Ident{Name: testingAlias},
				Sel: &ast.Ident{Name: "Test"},
			},
			Elts: elts,
		}
		cur.Replace(repl)
		hits++
		return false
	}, nil)

	if hits != 1 {
		t.Fatalf("generate.Params: Found %d testing.AddTest calls; want 1", hits)
	}

	var b bytes.Buffer
	if err := format.Node(&b, fs, root); err != nil {
		t.Fatalf("generate.Params: %v", err)
	}
	newCode := b.Bytes()

	// If updateEnv is set, update the source code.
	if os.Getenv(updateEnv) == "1" {
		if err := ioutil.WriteFile(file, newCode, 0666); err != nil {
			t.Fatalf("Failed to save the generated code: %v", err)
		}
		return
	}

	if !bytes.Equal(newCode, oldCode) {
		pkg := strings.Split(caller.Get(2), ".")[0]
		t.Errorf("Params is stale; run the following command to update:\n  %s=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 %s", updateEnv, pkg)
	}
}
