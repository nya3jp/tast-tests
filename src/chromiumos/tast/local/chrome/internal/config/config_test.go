// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"reflect"
	"strings"
	gotesting "testing"
)

func TestReuseTag(t *gotesting.T) {
	cfg, _ := NewConfig(nil)
	tp := reflect.TypeOf(&cfg.m).Elem()
	// Iterate over all available fields and verify the "reuse_match" tag value.
	for i := 0; i < tp.NumField(); i++ {
		field := tp.Field(i)

		// Get the field tag.
		reuseMatch := field.Tag.Get("reuse_match")

		wanted := []string{"false", "true", "customized"}

		found := false
		for _, v := range wanted {
			if v == reuseMatch {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("reuse_match tag for field %q has unexpected value %q; want one of: %v", field.Name, reuseMatch, wanted)
		}
	}
}

func TestVerifySessionReuse(t *gotesting.T) {
	checkErrorContains := func(err error, keyWord string) {
		t.Helper()
		if err == nil {
			t.Errorf("No error is given when key word %q is expected", keyWord)
			return
		}
		if !strings.Contains(err.Error(), keyWord) {
			t.Errorf("Error has no information about key word %q. Got: %v", keyWord, err)
		}
	}
	// Verify field with `reuse_match:"false"` tag can be different.
	cfg1, _ := NewConfig([]Option{func(cfg *MutableConfig) error { cfg.KeepState = false; return nil }})
	cfg2, _ := NewConfig([]Option{func(cfg *MutableConfig) error { cfg.KeepState = true; return nil }})
	if err := cfg1.VerifySessionReuse(cfg2); err != nil {
		t.Errorf("Reuse should be allowed with different KeepState values. Got: %v", err)
	}

	// Verify field without "reuse_match" tag must exactly match.
	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.Creds.User = "user1@example.com"; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.Creds.User = "user2@example.com"; return nil }})
	err := cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed with different User values")
	} else {
		checkErrorContains(err, "Creds")
	}

	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.Creds.User = "user1@example.com"; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.Creds.User = "user1@example.com"; return nil }})
	if err = cfg1.VerifySessionReuse(cfg2); err != nil {
		t.Errorf("Reuse should be allowed with same User values. Got: %v", err)
	}

	// Verify field with `reuse_match:"customized"` tag: DeferLogin.
	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.DeferLogin = true; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.DeferLogin = true; return nil }})
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when DeferLogin is true")
	} else {
		checkErrorContains(err, "DeferLogin")
	}

	// Verify field with `reuse_match:"customized"` tag: LoginMode.
	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.LoginMode = NoLogin; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.LoginMode = NoLogin; return nil }})
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when LoginMode mode is NoLogin")
	} else {
		checkErrorContains(err, "NoLogin")
	}

	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.LoginMode = GAIALogin; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.LoginMode = NoLogin; return nil }})
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when LoginMode is different")
	} else {
		checkErrorContains(err, "LoginMode")
	}

	// Verify fields with type []string must match.
	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.ExtraArgs = []string{"a", "b"}; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.ExtraArgs = []string{"a", "b"}; return nil }})
	if cfg1.VerifySessionReuse(cfg2) != nil {
		t.Errorf("Reuse should be allowed when ExtraArgs are the same. Got: %v", err)
	}
	cfg1, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.EnableFeatures = []string{"a", "b"}; return nil }})
	cfg2, _ = NewConfig([]Option{func(cfg *MutableConfig) error { cfg.EnableFeatures = []string{"b"}; return nil }})
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when EnableFeatures are different")
	} else {
		checkErrorContains(err, "EnableFeatures")
	}
}
