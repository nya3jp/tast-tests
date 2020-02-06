// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/json"
	"testing"
)

func TestMessageEqual(t *testing.T) {
	tcs := []struct {
		m      json.RawMessage
		p      Policy
		result bool
		isErr  bool
	}{
		// Type: bool.
		{json.RawMessage("true"), &AllowDinosaurEasterEgg{Val: true}, true, false},
		{json.RawMessage("false"), &AllowDinosaurEasterEgg{Val: false}, true, false},
		{json.RawMessage("true"), &AllowDinosaurEasterEgg{Val: false}, false, false},
		{json.RawMessage(""), &AllowDinosaurEasterEgg{Val: true}, false, true},
		// Type: int.
		{json.RawMessage("1"), &DeviceUpdateScatterFactor{Val: 1}, true, false},
		{json.RawMessage("-3"), &DeviceUpdateScatterFactor{Val: -3}, true, false},
		{json.RawMessage("0"), &DeviceUpdateScatterFactor{Val: 7}, false, false},
		{json.RawMessage(""), &DeviceUpdateScatterFactor{Val: 0}, false, true},
		// Type: string.
		{json.RawMessage("\"asdf\""), &HomepageLocation{Val: "asdf"}, true, false},
		{json.RawMessage("\"foo\""), &HomepageLocation{Val: "bar"}, false, false},
		{json.RawMessage("{\"object\": 4}"), &HomepageLocation{Val: "str"}, false, true},
		// Type: []string.
		{json.RawMessage("[\"asdf\", \"foo\"]"),
			&DisabledSchemes{Val: []string{"asdf", "foo"}}, true, false},
		{json.RawMessage("[\"one\", \"two\"]"),
			&DisabledSchemes{Val: []string{"one", "three"}}, false, false},
		{json.RawMessage("[\"one\", \"two\"]"),
			&DisabledSchemes{Val: []string{"one", "two", "three"}}, false, false},
		{json.RawMessage("\"str\""),
			&DisabledSchemes{Val: []string{"str"}}, false, true},
		// Type: object.
		{json.RawMessage("{\"url\":\"https://example.com/wallpaper.jpg\",\"hash\":\"baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf\"}"),
			&WallpaperImage{Val: &WallpaperImageValue{
				Url:  "https://example.com/wallpaper.jpg",
				Hash: "baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf"}},
			true, false},
		{json.RawMessage("{\"url\":\"https://example.com/badexample.jpg\",\"hash\":\"baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf\"}"),
			&WallpaperImage{Val: &WallpaperImageValue{
				Url:  "https://example.com/wallpaper.jpg",
				Hash: "baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf"}},
			false, false},
		{json.RawMessage("{\"badobject\":\"https://example.com/wallpaper.jpg\"}"),
			&WallpaperImage{Val: &WallpaperImageValue{
				Url:  "https://example.com/wallpaper.jpg",
				Hash: "baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf"}},
			false, false}, // Note that the UnmarshalAs command here will succeed.
		// Type: message with private field.
		{json.RawMessage("\"********\""), &PluginVmLicenseKey{Val: "foo"}, true, false},
		{json.RawMessage("\"foo\""), &PluginVmLicenseKey{Val: "foo"}, false, false},
	}
	for _, tc := range tcs {
		r, err := tc.p.UnmarshalAs(tc.m)
		if err != nil {
			if tc.isErr {
				continue
			}
			t.Errorf("Error unmarshalling %s as %v: %s", tc.m, tc.p.UntypedV(), err)
		}
		if tc.isErr {
			t.Errorf("Expected %s to fail to unmarshal as %v", tc.m, tc.p.UntypedV())
		}
		if cmp := tc.p.Equal(r); cmp != tc.result {
			t.Errorf("unexpected comparison between %s and %v", tc.m, tc.p.UntypedV())
		}
	}
}
