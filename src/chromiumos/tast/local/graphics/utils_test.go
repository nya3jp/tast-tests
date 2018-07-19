// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

// flagsSliceToMap converts a slice of strings into a map[string]struct{} where
// where the keys are the strings in the slice and the values are zero-byte
// values.
func flagsSliceToMap(s []string) map[string]struct{} {
	fmap := make(map[string]struct{})
	for _, flag := range s {
		fmap[flag] = struct{}{}
	}
	return fmap
}

func TestParseUIUseFlags(t *testing.T) {
	cases := []struct {
		name  string
		conf  string
		flags []string
	}{
		{"EmptyConf", "", []string{}},
		{"SingleFlag", "abc", []string{"abc"}},
		{"MultipleFlags", "abc\ndef", []string{"abc", "def"}},
		{"FlagsWithSpaces", "abc def\nGHi\n", []string{"abc def", "GHi"}},
		{"EmptyLines", "abc\n\n  \ndef\n\n", []string{"abc", "def"}},
		{"ExtraWhitespace", "abc\r\t\r\n\t  def  \t\t", []string{"abc", "def"}},
		{"CommentLines", "# c1\nabc\n#c2\ndef\n  # c3", []string{"abc", "def"}},
		{"OnlyComments", "# c1\n# c2\n# c3", []string{}},
		{"OnlyWhitespace", "   \n\t  \r\t\r\n\n\n", []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary file for the input configuration.
			f, err := ioutil.TempFile("", tc.name)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.WriteString(tc.conf)
			if err != nil {
				t.Fatal(err)
			}

			// Run parseUIUseFlags and compare result.
			expected := flagsSliceToMap(tc.flags)
			actual, err := parseUIUseFlags(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			if actual != nil {
				if !reflect.DeepEqual(expected, actual) {
					t.Errorf("Got flags = %q, expected %q", actual, expected)
				}
			} else {
				t.Errorf("Got flags = nil, expected %q", expected)
			}

			if err := os.Remove(f.Name()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
