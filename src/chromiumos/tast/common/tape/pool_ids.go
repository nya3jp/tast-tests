// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

// PoolIds for managed owned test accounts.
const (
	ArcEnterpriseLoginManaged3ppFalse     = "arc_enterprise_login_managed_3pp_false"
	ArcEnterpriseLoginManaged3ppTrue      = "arc_enterprise_login_managed_3pp_true"
	ArcEnterpriseLoginManagedNecktieFalse = "arc_enterprise_login_managed_necktie_false"
	ArcEnterpriseLoginManagedNecktieTrue  = "arc_enterprise_login_managed_necktie_true"
	ArcDataMigrationManaged               = "arc_data_migration_managed"
	ArcSnapshot                           = "arc_snapshot"
	ArcLoggingTest                        = "arc_logging_test"
	ChromeosbytebotCom                    = "chromeosbytebot_com"
	Crosprqa4Com                          = "crosprqa4_com"
	DefaultManaged                        = "default_managed"
	DeviceTrustDisabled                   = "device_trust_disabled"
	DeviceTrustEnabled                    = "device_trust_enabled"
	Enrollment                            = "enrollment"
	EnrollmentKiosk                       = "enrollment_kiosk"
	ZTETestAutomation                     = "zte-test-automation"
)

// PoolIds for unmanaged owned test accounts.
const (
	ArcDataMigrationUnmanaged               = "arc_data_migration_unmanaged"
	ArcEnterpriseLoginManagedUnmanagedFalse = "arc_enterprise_login_managed_unmanaged_false"
	ArcEnterpriseLoginManagedUnmanagedTrue  = "arc_enterprise_login_managed_unmanaged_true"
	DefaultUnmanaged                        = "default_unmanaged"
	UIDefault                               = "ui_default"
)

// PoolIds for Citrix accounts.
const (
	Citrix = "citrix"
)
