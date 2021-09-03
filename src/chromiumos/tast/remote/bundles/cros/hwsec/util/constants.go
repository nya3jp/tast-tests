// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

// This file contains some shared constants for vault/key related testing.
const (
	FirstUsername        = "pierredefermat@example.com"
	FirstPassword1       = "F131dTooSm@ll2C0nt@1nMyP@ssw0rd!!"
	FirstPassword2       = "F131dTooSm@ll2C0nt@1nMyP@ssw0rd2!!" // Some tests need a second key for the same user
	FirstChangedPassword = "a^n+b^n=c^n"                        // Got a great proof, but margin too small.
	FirstPin             = "65537"                              // 5th Fermat Number

	SecondUsername  = "leonhardeuler@example.com"
	SecondPassword1 = "e^(i*phi)=cos(phi)+i*sin(phi)"
	SecondPassword2 = "e^(i*theta)=cos(theta)+i*sin(theta)" // Some tests need a second key for the same user
	SecondPin       = "271828"                              // e

	ThirdUsername = "pythagoras@example.com"
	ThirdPassword = "a^2+b^2=c^2"

	Password1Label       = "password1"
	ChangedPasswordLabel = "changed"
	Password2Label       = "password2" // Some tests need a second key for the same user
	PinLabel             = "pin"

	IncorrectPassword = "ImJustGuessing~"

	TestFileName1   = "TESTFILE1"
	TestFileContent = "TEST_CONTENT"
)
