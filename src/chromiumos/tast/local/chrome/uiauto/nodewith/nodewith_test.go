// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nodewith

import (
	"regexp"
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
	for _, tc := range []struct {
		in  *Finder
		out string
	}{
		{Role(role.Button), `{}`},
		{ClassName("clsname"), `{"className":"clsname",}`},
		{HasClass("clsname"), `{"className":/\bclsname\b/,}`},
		{HasClass("clsname").NameRegex(regexp.MustCompile("^/nameBlah$")), `{"className":/\bclsname\b/,"name":selectName({"en":/^\/nameBlah$/,})}`},
		{Name("hello"),
			`{"name":selectName({"ar-XB":/^olleh$/,"en":/^hello$/,"en-XA":/^ĥéļļö one$/,})}`},
		{Name("/blah"), `{"name":selectName({"ar-XB":/^halb\/$/,"en":/^\/blah$/,"en-XA":/^\/bļåĥ one$/,})}`},
		{NameRegex(regexp.MustCompile("^/blah$")), `{"name":selectName({"en":/^\/blah$/,})}`},
		{NameRegex(regexp.MustCompile("(?i)^/blah$")), `{"name":selectName({"en":/^\/blah$/i,})}`},
		{NameRegex(regexp.MustCompile("^(?i)/blah$")), `{"name":selectName({"en":/^\/blah$/i,})}`},
		{MultilingualName("hello", map[string]string{"de": "hallo"}),
			`{"name":selectName({"ar-XB":/^olleh$/,"de":/^hallo$/,"en":/^hello$/,"en-XA":/^ĥéļļö one$/,})}`},
		{MultilingualName("hello", map[string]string{"en-XA": "hello a"}),
			`{"name":selectName({"ar-XB":/^olleh$/,"en":/^hello$/,"en-XA":/^hello a$/,})}`},
		{MultilingualNameStartingWith("hello", map[string]string{"de": "hallo", "ar": "abc"}),
			`{"name":selectName({"ar":/abc$/,"ar-XB":/olleh$/,"de":/^hallo/,"en":/^hello/,"en-XA":/^ĥéļļö/,})}`},
		{MultilingualNameContaining("hello", map[string]string{"de": "hallo", "ar": "abc"}),
			`{"name":selectName({"ar":/abc/,"ar-XB":/olleh/,"de":/hallo/,"en":/hello/,"en-XA":/ĥéļļö/,})}`},
	} {
		out, err := tc.in.attributesBytes()
		if err != nil {
			t.Errorf("%+v.attributesBytes() failed: %v", tc.in, err)
		} else if string(out) != tc.out {
			t.Errorf("%+v.attributesBytes() failed:\ngot:\n%v\nexpected:\n%v", tc.in, string(out), tc.out)
		}
	}
	if _, err := MultilingualName("hello", map[string]string{"en.us": "hello"}).attributesBytes(); err == nil {
		t.Error("MultilingualName should fail at converting to bytes when the language is invalid")
	}
}

func TestPrettyPrinter(t *gotesting.T) {
	for _, tc := range []struct {
		in  *Finder
		out string
	}{
		{newFinder(), `{}`},
		{Role(role.Button), `{role: button}`},
		{ClassName("clsname"), `{className: "clsname"}`},
		{HasClass("clsname"), `{className: /\bclsname\b/}`},
		{HasClass("clsname").NameRegex(regexp.MustCompile("^/nameBlah$")), `{name: /^/nameBlah$/, className: /\bclsname\b/}`},
		{Name("hello"), `{name: /^hello$/}`},
		{Collapsed(), `{state: map[collapsed:true]}`},
		{First(), `{first: true}`},
		{Nth(5), `{nth: 5}`},
		{Attribute("a", 5), `{a: 5}`},
		{Ancestor(Role(role.Button)), `{ancestor: {role: button}}`},
		{Name("hello").ClassName("cls"), `{name: /^hello$/, className: "cls"}`},
		{Name("hello").ClassName("cls").Ancestor(Role(role.Button)), `{name: /^hello$/, className: "cls", ancestor: {role: button}}`},
	} {
		out := tc.in.Pretty()
		if out != tc.out {
			t.Errorf("%+v.attributesBytes() failed:\ngot:\n%v\nexpected:\n%v", tc.in, string(out), tc.out)
		}
	}
}
