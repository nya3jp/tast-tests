// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"os"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
)

func TestErrorHolder(t *testing.T) {
	var eh ErrorHolder
	if eh.ToError() != nil {
		t.Fatal("The zero value of ErrorHolder should return nil error")
	}
}

func TestErrorHolderHold(t *testing.T) {
	var eh ErrorHolder
	eh.Hold(errors.New("test"))
	if eh.ToError() == nil {
		t.Fatal("ErrorHolder.Hold should hold the error")
	}
}

func TestErrorHolderHandle(t *testing.T) {
	var eh ErrorHolder
	if v := eh.Handle(func() (int, error) { return 1, errors.New("test") }()); v != 1 {
		t.Fatal("ErrorHolder.Handle should return the value")
	}
	if eh.ToError() == nil {
		t.Fatal("ErrorHolder.Handle should hold the error")
	}
}

func TestReadOptional(t *testing.T) {
	readFile = func(string) ([]byte, error) { return []byte("test\n"), nil }
	if v, _ := ReadOptionalStringFile(""); v == nil || *v != "test" {
		t.Fatal("ReadOptionalStringFile failed to read file, got:", v)
	}
	readFile = func(string) ([]byte, error) { return nil, errors.New("test") }
	if _, err := ReadOptionalStringFile(""); err == nil {
		t.Fatal("ReadOptionalStringFile should return error")
	}
	readFile = func(string) ([]byte, error) { return nil, errors.Wrap(os.ErrNotExist, "test") }
	if _, err := ReadOptionalStringFile(""); err != nil {
		t.Fatal("ReadOptionalStringFile should not return ErrNotExist")
	}
}

func TestOptionalCrosConfig(t *testing.T) {
	getCrosConfig = func(context.Context, string, string) (string, error) { return "test", nil }
	if v, _ := GetOptionalCrosConfig(nil, ""); v == nil || *v != "test" {
		t.Fatal("GetOptionalCrosConfig failed to read file, got:", v)
	}
	getCrosConfig = func(context.Context, string, string) (string, error) { return "", errors.New("test") }
	if _, err := GetOptionalCrosConfig(nil, ""); err == nil {
		t.Fatal("GetOptionalCrosConfig should return error")
	}
	getCrosConfig = func(context.Context, string, string) (string, error) {
		return "", errors.Wrap(&crosconfig.ErrNotFound{E: errors.New("test")}, "test")
	}
	if _, err := GetOptionalCrosConfig(nil, ""); err != nil {
		t.Fatal("GetOptionalCrosConfig should not return ErrNotExist")
	}
}

func TestIsCrosConfigTrue(t *testing.T) {
	getCrosConfig = func(context.Context, string, string) (string, error) { return "true", nil }
	if v, _ := IsCrosConfigTrue(nil, ""); !v {
		t.Fatal("IsCrosConfigTrue should return true")
	}
	getCrosConfig = func(context.Context, string, string) (string, error) { return "false", nil }
	if v, _ := IsCrosConfigTrue(nil, ""); v {
		t.Fatal("IsCrosConfigTrue should return false")
	}
	getCrosConfig = func(context.Context, string, string) (string, error) { return "", errors.New("test") }
	if _, err := IsCrosConfigTrue(nil, ""); err == nil {
		t.Fatal("IsCrosConfigTrue should return error")
	}
	getCrosConfig = func(context.Context, string, string) (string, error) {
		return "", errors.Wrap(&crosconfig.ErrNotFound{E: errors.New("test")}, "test")
	}
	if v, err := IsCrosConfigTrue(nil, ""); v || err != nil {
		t.Fatal("IsCrosConfigTrue should return false and should not return error")
	}
}
