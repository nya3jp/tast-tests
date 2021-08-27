// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kerberos contains details about Kerberos setup that is used in
// testing.
package kerberos

const (
	// KerberosDomain domain of the hosting server.
	KerberosDomain = "NOT_SET_validDomainNeedsToBeProvided"
	// ServerAllowList server name for integrated authorization.
	ServerAllowList = "*" + KerberosDomain
	// WebsiteAddress website address that is guarded by Kerberos.
	WebsiteAddress = "https://" + KerberosDomain
	// KerberosUser user name.
	KerberosUser = "test@" + KerberosDomain
	// KerberosUserPass password.
	KerberosUserPass = "NOT_SET_validPasswordNeedsToBeProvided"
	// RemoteFileSystemURI is a identifier of remote file system that is
	// guarded by Kerberos.
	RemoteFileSystemURI = "\\\\chromed-server-windows2012." + KerberosDomain + "\\sysvol"
)
