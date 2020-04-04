// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     V2Compat,
		Desc:     "Verifies V2 compatibility",
		Contacts: []string{"tast-owners@google.com"},
		// Manual test.
		Attr: []string{},
	})
}

var numRE = regexp.MustCompile(`(\d+|0x[\dA-Fa-f]+)(ms |s )?`)

var blackList = []string{
	"Command line: ",
	"Connecting to browser at ws:",
	"Connecting to Chrome target ",
}

func normalize(o string) []string {
	ss := strings.Split(o, "\n")
	var res []string
outer:
	for _, s := range ss {
		for _, b := range blackList {
			if strings.Contains(s, b) {
				continue outer
			}
		}
		s = numRE.ReplaceAllString(s, "X")
		res = append(res, s)
	}
	return res
}

func diff(o1, o2 string) string {
	n1 := normalize(o1)
	n2 := normalize(o2)

	return cmp.Diff(n1, n2)
}

func getV1(ctx context.Context, expr string) (string, error) {
	path := filepath.Join("/tmp/hoge/", expr)
	if b, err := ioutil.ReadFile(path); err == nil {
		return string(b), nil
	}
	b, err := testexec.CommandContext(ctx, "/home/oka/go/bin/tast", "run", "localhost:9222", expr).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	os.MkdirAll(filepath.Dir(path), 0755)
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		return "", err
	}
	return string(b), nil
}

func getV2(ctx context.Context, expr string) (string, error) {
	b, err := testexec.CommandContext(ctx, "/home/oka/go/bin/tast", "run", "-v2", "localhost:9222", expr).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func V2Compat(ctx context.Context, s *testing.State) {
	for _, expr := range []string{
		"example.Pass",
		"example.Keyboard",
	} {
		want, err := getV1(ctx, expr)
		if err != nil {
			s.Fatal("cmd1: ", err)
		}
		got, err := getV2(ctx, expr)
		if err != nil {
			s.Fatal("cmd2: ", err)
		}
		if d := diff(got, want); d != "" {
			s.Errorf("expr %s; diff (-got +want): %v", expr, d)
		}
	}
}
