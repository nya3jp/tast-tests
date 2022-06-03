// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kerberos contains details about Kerberos setup that is used in
// testing.
package kerberos

const (
	protocol  = "https"
	subdomain = "dc1"
	folder    = "kerberostest"
	file      = "test.txt"
)

// ConstructConfig prepares all necessary constants that are used by tests.
func ConstructConfig(kerberosDomain, username string) *Configuration {

	fullDomain := subdomain + "." + kerberosDomain
	return &Configuration{
		KerberosDomain:      kerberosDomain,
		ServerAllowlist:     "*" + kerberosDomain,
		WebsiteAddress:      protocol + "://" + fullDomain,
		KerberosAccount:     username + "@" + kerberosDomain,
		Folder:              folder,
		RemoteFileSystemURI: "\\\\" + fullDomain + "\\" + folder,
		File:                file,
		RealmsConfig:        "\n[realms]\nKER.CAPSE-ISS-AD.COM = {\nkdc = " + fullDomain + "\nmaster_kdc = " + fullDomain + "\n}", // NOLINT //nocheck
	}
}

// Configuration contains all necessary data needed for test to access
// Kerberos infrastructure.
type Configuration struct {
	// KerberosDomain domain of the hosting server.
	KerberosDomain string
	// ServerAllowlist server name for integrated authorization.
	ServerAllowlist string
	// WebsiteAddress website address that is guarded by Kerberos.
	WebsiteAddress string
	// KerberosAccount is constructed in a following way username@domain.
	KerberosAccount string
	// Folder is the name of the directory that is part of RemoteFileSystemURI.
	Folder string
	// RemoteFileSystemURI is a identifier of remote file system that is
	// guarded by Kerberos.
	RemoteFileSystemURI string
	// File is the name of the file that is expected on samba mount.
	File string
	// RealmsConfig is an advanced configuration that helps finding the kdc on the KerberosDomain.
	RealmsConfig string
}
