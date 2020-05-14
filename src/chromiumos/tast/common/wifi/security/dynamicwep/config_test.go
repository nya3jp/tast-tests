// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dynamicwep

import (
	"reflect"
	"testing"

	"chromiumos/tast/common/wifi/security/eap"
)

func TestDynamicWEP(t *testing.T) {
	// Calling without option to check default values.
	if _, err := NewConfigFactory("CA", "Cert", "Key").Gen(); err != nil {
		t.Error("failed to Gen with default values")
	}

	fac := NewConfigFactory(
		"ServerCACert", "ServerCert", "ServerKey",
		RekeyPeriod(10),
		UseShortKey(),
		FileSuffix("FileSuffix"),
		ClientCACert("ClientCACert"),
		ClientCert("ClientCert"),
		ClientKey("ClientKey"),
		ClientCertID("8888"),
		ClientKeyID("9999"),
	)
	eapFac := eap.NewConfigFactory(
		"ServerCACert", "ServerCert", "ServerKey",
		eap.FileSuffix("FileSuffix"),
		eap.ClientCACert("ClientCACert"),
		eap.ClientCert("ClientCert"),
		eap.ClientKey("ClientKey"),
		eap.ClientCertID("8888"),
		eap.ClientKeyID("9999"),
	)
	expectedConf := &Config{
		useShortKey: true,
		rekeyPeriod: 10,
	}

	confInterface, err := fac.Gen()
	if err != nil {
		t.Error("failed to Gen Config")
	}
	eapConfInterface, err := eapFac.Gen()
	if err != nil {
		t.Fatal("falied to Gen eap.Config, there should be a bug in eap package")
	}
	conf := confInterface.(*Config)
	expectedConf.Config = eapConfInterface.(*eap.Config)
	if !reflect.DeepEqual(conf, expectedConf) {
		t.Errorf("got %v, want %v", conf, expectedConf)
	}

	// Since we are not able to modify the private fields in eap.Config without using unsafe package,
	// it seems not possible to check hosapd config and shill properties in this package.
}
