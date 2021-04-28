// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nodewith

import (
	gotesting "testing"

	"chromiumos/tast/local/chrome/uiauto/role"
)

func TestArXbPseudolocale(t *gotesting.T) {
	if pseudo := makeArXBString("abc"); pseudo != "cba" {
		t.Errorf("Unexpected ar-XB string: got %+v, want \"cba\"", pseudo)
	}
	if pseudo := makeArXBString("abcd"); pseudo != "dcba" {
		t.Errorf("Unexpected ar-XB string: got %+v, want \"dcba\"", pseudo)
	}
	if pseudo := makeArXBString("平仮名"); pseudo != "名仮平" {
		t.Errorf("Unexpected ar-XB string: got %+v, want \"名仮平\"", pseudo)
	}
}

func TestEnXaPseudolocale(t *gotesting.T) {
	if pseudo := makeEnXAString("bbb", true); pseudo != "bbb one" {
		t.Errorf("Unexpected en-XA string: got %+v, want \"bbb one\"", pseudo)
	}

	if pseudo := makeEnXAString("a", false); pseudo != "å" {
		t.Errorf("Unexpected en-XA string: got %+v, want \"å\"", pseudo)
	}

	long := "b b b b b b b b b b b one two three four five six seven eight nine ten one"
	if pseudo := makeEnXAString("b b b b b b b b b b b", true); pseudo != long {
		t.Errorf("Unexpected en-XA string: got %+v, want %+v", pseudo, long)
	}

	allPseudoLetters := "åbçðéƒĝĥîĵķļmñöþqršţûvŵxýžÅBÇÐÉFĜĤÎĴĶĻMÑÖÞQ®ŠŢÛVŴXÝŽ¡@#€%^&*()¿"
	if pseudo := makeEnXAString("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()?", false); pseudo != allPseudoLetters {
		t.Errorf("Unexpected en-XA string: got %+v, want %v", pseudo, allPseudoLetters)
	}
}

func TestAttributeMatcher(t *gotesting.T) {
	// Empty out indicates an error is expected.
	for _, tc := range []struct {
		in  *Finder
		out string
	}{
		{Role(role.Button), `{}`},
		{ClassName("clsname"), `{"className":"clsname",}`},
		{Name("hello"),
			`{"name":selectName({"ar-XB":/^olleh$/,"en":/^hello$/,"en-XA":/^ĥéļļö one$/,})}`},
		{MultilingualName("hello", map[string]string{"de": "hallo"}),
			`{"name":selectName({"ar-XB":/^olleh$/,"de":/^hallo$/,"en":/^hello$/,"en-XA":/^ĥéļļö one$/,})}`},
		{MultilingualName("hello", map[string]string{"en-XA": "hello a"}),
			`{"name":selectName({"ar-XB":/^olleh$/,"en":/^hello$/,"en-XA":/^hello a$/,})}`},
		{MultilingualNameStartingWith("hello", map[string]string{"de": "hallo", "ar": "abc"}),
			`{"name":selectName({"ar":/^.*abc$/,"ar-XB":/^.*olleh$/,"de":/^hallo.*$/,"en":/^hello.*$/,"en-XA":/^ĥéļļö.*$/,})}`},
		{MultilingualNameContaining("hello", map[string]string{"de": "hallo", "ar": "abc"}),
			`{"name":selectName({"ar":/^.*abc.*$/,"ar-XB":/^.*olleh.*$/,"de":/^.*hallo.*$/,"en":/^.*hello.*$/,"en-XA":/^.*ĥéļļö.*$/,})}`},
	} {
		out, err := tc.in.attributesBytes()
		if err != nil {
			t.Errorf("%+v.attributesBytes() failed: %v", tc.in, err)
		} else if string(out) != tc.out {
			t.Errorf("%+v.attributesBytes() failed:\ngot:\n%v\nexpected:\n%v", tc.in, string(out), tc.out)
		}
	}
}
