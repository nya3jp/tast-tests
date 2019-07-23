// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kerberos interacts with the Kerberos system daemon.
package kerberos

import (
	"context"
	"os"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	kp "chromiumos/system_api/kerberos_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.Kerberos"
	dbusPath      = "/org/chromium/Kerberos"
	dbusInterface = "org.chromium.Kerberos"
)

// Kerberos is used to interact with the Kerberos daemon process over D-Bus.
// For documentation of the D-Bus methods, see
// src/platform2/kerberos/dbus_bindings/org.chromium.Kerberos.xml.
type Kerberos struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// New connects to the Kerberos daemon via D-Bus and returns a Kerberos object.
func New(ctx context.Context) (*Kerberos, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &Kerberos{conn, obj}, nil
}

// AddAccount calls Kerberos.AddAccount D-Bus method.
func (k *Kerberos) AddAccount(ctx context.Context, principalName string) (*kp.AddAccountResponse, error) {
	request := &kp.AddAccountRequest{PrincipalName: &principalName}
	response := &kp.AddAccountResponse{}
	err := k.callProtoMethod(ctx, "AddAccount", request, response)
	return response, err
}

// RemoveAccount calls Kerberos.RemoveAccount D-Bus method.
func (k *Kerberos) RemoveAccount(ctx context.Context, principalName string) (*kp.RemoveAccountResponse, error) {
	request := &kp.RemoveAccountRequest{PrincipalName: &principalName}
	response := &kp.RemoveAccountResponse{}
	err := k.callProtoMethod(ctx, "RemoveAccount", request, response)
	return response, err
}

// ClearAccounts calls Kerberos.ClearAccounts D-Bus method.
func (k *Kerberos) ClearAccounts(ctx context.Context) (*kp.ClearAccountsResponse, error) {
	request := &kp.ClearAccountsRequest{}
	response := &kp.ClearAccountsResponse{}
	err := k.callProtoMethod(ctx, "ClearAccounts", request, response)
	return response, err
}

// ListAccounts calls Kerberos.ListAccounts D-Bus method.
func (k *Kerberos) ListAccounts(ctx context.Context) (*kp.ListAccountsResponse, error) {
	request := &kp.ListAccountsRequest{}
	response := &kp.ListAccountsResponse{}
	err := k.callProtoMethod(ctx, "ListAccounts", request, response)
	return response, err
}

// SetConfig calls Kerberos.SetConfig D-Bus method.
func (k *Kerberos) SetConfig(ctx context.Context, principalName, krb5Conf string) (*kp.SetConfigResponse, error) {
	request := &kp.SetConfigRequest{PrincipalName: &principalName, Krb5Conf: &krb5Conf}
	response := &kp.SetConfigResponse{}
	err := k.callProtoMethod(ctx, "SetConfig", request, response)
	return response, err
}

// ValidateConfig calls Kerberos.ValidateConfig D-Bus method.
func (k *Kerberos) ValidateConfig(ctx context.Context, krb5Conf string) (*kp.ValidateConfigResponse, error) {
	request := &kp.ValidateConfigRequest{Krb5Conf: &krb5Conf}
	response := &kp.ValidateConfigResponse{}
	err := k.callProtoMethod(ctx, "ValidateConfig", request, response)
	return response, err
}

// AcquireKerberosTgt calls Kerberos.AcquireKerberosTgt D-Bus method.
func (k *Kerberos) AcquireKerberosTgt(ctx context.Context, principalName, password string, rememberPassword, useLoginPassword bool) (*kp.AcquireKerberosTgtResponse, error) {
	method := dbusInterface + "." + "AcquireKerberosTgt"
	request := &kp.AcquireKerberosTgtRequest{
		PrincipalName:    &principalName,
		RememberPassword: &rememberPassword,
		UseLoginPassword: &useLoginPassword,
	}
	response := &kp.AcquireKerberosTgtResponse{}

	// Can't use callProtoMethod since AcquireKerberosTgt() takes the password
	// as an extra file descriptor arg.
	marshRequest, err := proto.Marshal(request)
	if err != nil {
		return response, errors.Wrapf(err, "failed marshaling %s request", method)
	}

	passwordPipe, err := writeStringToPipe(password)
	if err != nil {
		return response, errors.Wrapf(err, "failed writing %s password to pipe", method)
	}
	defer passwordPipe.Close()

	call := k.obj.CallWithContext(ctx, method, 0, marshRequest, dbus.UnixFD(passwordPipe.Fd()))
	if call.Err != nil {
		return response, errors.Wrapf(call.Err, "failed calling %s", method)
	}
	var marshResponse []byte
	if err := call.Store(&marshResponse); err != nil {
		return response, errors.Wrapf(err, "failed reading %s response", method)
	}
	if err := proto.Unmarshal(marshResponse, response); err != nil {
		return response, errors.Wrapf(err, "failed unmarshaling %s response", method)
	}
	return response, nil
}

// GetKerberosFiles calls Kerberos.GetKerberosFiles D-Bus method.
func (k *Kerberos) GetKerberosFiles(ctx context.Context, principalName string) (*kp.GetKerberosFilesResponse, error) {
	request := &kp.GetKerberosFilesRequest{PrincipalName: &principalName}
	response := &kp.GetKerberosFilesResponse{}
	err := k.callProtoMethod(ctx, "GetKerberosFiles", request, response)
	return response, err
}

// callProtoMethod is thin wrapper of CallProtoMethod for convenience.
func (k *Kerberos) callProtoMethod(ctx context.Context, method string, in, out proto.Message) error {
	return dbusutil.CallProtoMethod(ctx, k.obj, dbusInterface+"."+method, in, out)
}

// writeStringToPipe writes |str| to a pipe and returns the reading end.
func writeStringToPipe(str string) (*os.File, error) {
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	pipeW.WriteString(str)
	pipeW.Close()
	return pipeR, nil
}
