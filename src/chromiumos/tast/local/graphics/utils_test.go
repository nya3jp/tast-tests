// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
)

// flagsStringToMap converts a space-separated list of flags into a
// map[string]struct{} where the keys are the flags in the string and the values
// are zero-byte values.
func flagsStringToMap(s string) map[string]struct{} {
	fmap := make(map[string]struct{})
	for _, flag := range strings.Fields(s) {
		fmap[flag] = struct{}{}
	}
	return fmap
}

func TestParseUIUseFlags(t *testing.T) {
	for _, tc := range []struct {
		name  string
		conf  string
		flags string
	}{
		{"EmptyConf", "", ""},
		{"SingleFlag", "abc", "abc"},
		{"MultipleFlags", "abc\ndef", "abc def"},
		{"EmptyLines", "abc\n\n  \ndef\n\n", "abc def"},
		{"ExtraWhitespace", "abc\r\t\r\n\t  def  \t\t", "abc def"},
		{"CommentLines", "# c1\nabc\n#c2\ndef\n  # c3", "abc def"},
		{"OnlyComments", "# c1\n# c2\n# c3", ""},
		{"OnlyWhitespace", "   \n\t  \r\t\r\n\n\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary file for the input configuration.
			f, err := ioutil.TempFile("", tc.name)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(f.Name()); err != nil {
					t.Fatal(err)
				}
			}()
			if _, err = f.WriteString(tc.conf); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}

			// Run parseUIUseFlags and compare result.
			expected := flagsStringToMap(tc.flags)
			actual, err := parseUIUseFlags(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			if actual != nil {
				if !reflect.DeepEqual(expected, actual) {
					t.Errorf("parseUIUseFlags on %q = %v; want %v", tc.conf,
						actual, expected)
				}
			} else {
				t.Errorf("parseUIUseFlags on %q = nil; want %v", tc.conf,
					expected)
			}
		})
	}
}
