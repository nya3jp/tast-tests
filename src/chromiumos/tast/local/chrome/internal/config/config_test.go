// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	gotesting "testing"
)

func TestIsSessionReusable(t *gotesting.T) {
	// Verify field without "reuse_match" tag will be ignored.
	cfg1, _ := NewConfig(nil)
	cfg2, _ := NewConfig(nil)
	cfg1.KeepState = false
	cfg2.KeepState = true
	if !cfg1.IsSessionReusable(cfg2) {
		t.Error("Reuse should be true with different KeepState values")
	}

	// Verify field with `reuse_match:"true"` tag will be counted.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.User = "user1@example.com"
	cfg2.User = "user2@example.com"
	if cfg1.IsSessionReusable(cfg2) {
		t.Error("Reuse should be false with different User values")
	}

	// Verify field with `reuse_match:"explicit"` tag: DeferLogin.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.DeferLogin = true
	cfg2.DeferLogin = true
	if cfg1.IsSessionReusable(cfg2) {
		t.Error("Reuse should be false when DeferLogin is true")
	}

	// Verify field with `reuse_match:"explicit"` tag: LoginMode.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.LoginMode = NoLogin
	cfg2.LoginMode = NoLogin
	if cfg1.IsSessionReusable(cfg2) {
		t.Error("Reuse should be false when LoginMode mode is NoLogin")
	}
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.LoginMode = GAIALogin
	cfg2.LoginMode = GuestLogin
	if cfg1.IsSessionReusable(cfg2) {
		t.Error("Reuse should be false when LoginMode is different")
	}

	// Verify field with `reuse_match:"explicit"` tag: ExtraArgs.
	cfg1, _ = NewConfig(nil)
	cfg2, _ = NewConfig(nil)
	cfg1.ExtraArgs = []string{"a", "b"}
	cfg2.ExtraArgs = []string{"b", "a"}
	cfg1.EnableFeatures = []string{"a", "b"}
	cfg2.EnableFeatures = []string{"b", "a"}
	cfg1.DisableFeatures = []string{"a", "b"}
	cfg2.DisableFeatures = []string{"b", "a"}
	if !cfg1.IsSessionReusable(cfg2) {
		t.Error("Reuse should be true when ExtraArgs, EnableFeatures, and DisableFeatures match")
	}
}
