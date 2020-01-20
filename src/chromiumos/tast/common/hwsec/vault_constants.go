// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

// This file contains some shared constants for vault/key related testing.
const (
	FirstUsername        = "PierreDeFermat@example.com"
	FirstPassword        = "F131dTooSm@ll2C0nt@1nMyP@ssw0rd!!"
	FirstChangedPassword = "a^n+b^n=c^n" // Got a great proof, but margin too small.
	FirstPin             = "65537"       // 5th Fermat Number

	SecondUsername = "LeonhardEuler@example.com"
	SecondPassword = "e^(i*phi)=cos(phi)+i*sin(phi)"
	SecondPin      = "271828" // e
	ThirdUsername  = "Pythagoras@example.com"
	ThirdPassword  = "a^2+b^2=c^2"

	PasswordLabel        = "password"
	ChangedPasswordLabel = "changed"
	PinLabel             = "pin"
	IncorrectPassword    = "ImJustGuessing~"
)
