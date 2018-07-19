// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
	// Create a temporary directory for input configuration files.
	tmpd, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpd)

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
			f := filepath.Join(tmpd, tc.name)
			if err = ioutil.WriteFile(f, []byte(tc.conf), 0600); err != nil {
				t.Fatal(err)
			}

			// Run parseUIUseFlags and compare result.
			expected := flagsStringToMap(tc.flags)
			actual, err := parseUIUseFlags(f)
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

func TestExtractOpenGLVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		wflout  string
		major   int
		minor   int
		wantErr bool
	}{
		{"EmptyOutput", "", 0, 0, true},
		{"TypicalOutput", `
Waffle platform: null
Waffle api: gles2
OpenGL vendor string: Intel Open Source Technology Center
OpenGL renderer string: Mesa DRI Intel(R) HD Graphics 615 (Kaby Lake GT2)
OpenGL version string: OpenGL ES 3.2 Mesa 18.1.0-devel (git-131e871385)`, 3, 2, false},
		{"DuplicateVersionString", `
Waffle platform: null
Waffle api: gles2
OpenGL vendor string: Intel Open Source Technology Center
OpenGL version string: OpenGL ES 3.3 Mesa 18.1.0-devel (git-131e871385)
OpenGL renderer string: Mesa DRI Intel(R) HD Graphics 615 (Kaby Lake GT2)
OpenGL version string: OpenGL ES 3.2 Mesa 18.1.0-devel (git-131e871385)`, 0, 0, true},
		{"NoVersionString", `
Waffle platform: null
Waffle api: gles2
OpenGL vendor string: Intel Open Source Technology Center`, 0, 0, true},
		{"BadMajorVersion", "OpenGL version string: OpenGL ES 999999999999999999999.2 Mesa 18.1.0-devel (git-131e871385)", 0, 0, true},
		{"BadMinorVersion", "OpenGL version string: OpenGL ES 3.999999999999999999999 Mesa 18.1.0-devel (git-131e871385)", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amajor, aminor, aerr := extractOpenGLVersion(context.Background(),
				strings.TrimLeft(tc.wflout, "\n"))

			// Complain if a) no error was returned, but we expected one, or
			// b) an error was returned, but we expected none, or c) the
			// returned major and minor versions are not as expected.
			if tc.wantErr {
				if aerr == nil {
					t.Errorf("extractOpenGLVersion(%q) unexpectedly succeeded",
						tc.wflout)
				}
				return
			}

			if aerr != nil {
				t.Errorf("extractOpenGLVersion(%q) unexpectedly failed: %v",
					tc.wflout, aerr)
			} else if amajor != tc.major || aminor != tc.minor {
				t.Errorf("extractOpenGLVersion(%q) = [%v, %v]; want [%v, %v]",
					tc.wflout, amajor, aminor, tc.major, tc.minor)
			}
		})
	}
}
