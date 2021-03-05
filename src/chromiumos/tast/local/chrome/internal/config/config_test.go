// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	"strings"
	gotesting "testing"
)

func TestReuseTag(t *gotesting.T) {
	cfg, err := NewConfig(nil)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	// In case of invalid reuse_match tag usage, VerifySessionReuse will
	// panic. We're not interested in its return value.
	_ = cfg.VerifySessionReuse(cfg)
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
	cfg1, _ := NewConfig(nil)
	cfg2, _ := NewConfig(nil)
	cfg1.KeepState = false
	cfg2.KeepState = true
	if err := cfg1.VerifySessionReuse(cfg2); err != nil {
		t.Errorf("Reuse should be allowed with different KeepState values. Got: %v", err)
	}

	// Verify field without "reuse_match" tag must exactly match.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.Creds.User = "user1@example.com"
	cfg2.Creds.User = "user2@example.com"
	err := cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed with different User values")
	} else {
		checkErrorContains(err, "Creds")
	}

	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.Creds.User = "user1@example.com"
	cfg2.Creds.User = "user1@example.com"
	if err = cfg1.VerifySessionReuse(cfg2); err != nil {
		t.Errorf("Reuse should be allowed with same User values. Got: %v", err)
	}

	// Verify field with `reuse_match:"customized"` tag: DeferLogin.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.DeferLogin = true
	cfg2.DeferLogin = true
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when DeferLogin is true")
	} else {
		checkErrorContains(err, "DeferLogin")
	}

	// Verify field with `reuse_match:"customized"` tag: LoginMode.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.LoginMode = NoLogin
	cfg2.LoginMode = NoLogin
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when LoginMode mode is NoLogin")
	} else {
		checkErrorContains(err, "NoLogin")
	}

	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.LoginMode = GAIALogin
	cfg2.LoginMode = GuestLogin
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when LoginMode is different")
	} else {
		checkErrorContains(err, "LoginMode")
	}

	// Verify fields with type []string must match.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.ExtraArgs = []string{"a", "b"}
	cfg2.ExtraArgs = []string{"a", "b"}
	if cfg1.VerifySessionReuse(cfg2) != nil {
		t.Errorf("Reuse should be allowed when ExtraArgs are the same. Got: %v", err)
	}
	cfg1.EnableFeatures = []string{"a", "b"}
	cfg2.EnableFeatures = []string{"b"}
	err = cfg1.VerifySessionReuse(cfg2)
	if err == nil {
		t.Error("Reuse should not be allowed when EnableFeatures are different")
	} else {
		checkErrorContains(err, "EnableFeatures")
	}
}
