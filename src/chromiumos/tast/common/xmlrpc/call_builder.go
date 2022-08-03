// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package xmlrpc

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
)

// RPCInterface is an interface that provides a common way of making RPC calls.
type RPCInterface interface {
	// RPC returns a new CallBuilder instance with the CallBuilder.Name and
	// CallBuilder.NamePrefix already set with methodName and a prefix handled
	// by the implementation, respectively.
	//
	// Calling this method does not preform a Remote-Procedure-Call itself. The
	// CallBuilder.Call method from the returned CallBuilder instance can be used
	// to do so.
	RPC(methodName string) *CallBuilder

	// Host returns the host and port of the device this RPC sends requests to.
	Host() string
}

// CommonRPCInterface is a base implementation of RPCInterface which can be
// included in structs that wish to implement RPCInterface in a common way with
// XMLRpc as the XMLRPC client and an optional methodNamePrefix.
type CommonRPCInterface struct {
	XMLRPC                 *XMLRpc
	XMLRPCMethodNamePrefix string
}

// NewCommonRPCInterface creates a new instance of CommonRPCInterface.
func NewCommonRPCInterface(xmlrpcClient *XMLRpc, methodNamePrefix string) *CommonRPCInterface {
	return &CommonRPCInterface{
		XMLRPC:                 xmlrpcClient,
		XMLRPCMethodNamePrefix: methodNamePrefix,
	}
}

// RPC returns a new CallBuilder instance with the CallBuilder.Name and
// CallBuilder.NamePrefix already set with methodName and a prefix handled
// by the implementation, respectively.
//
// Calling this method does not preform a Remote-Procedure-Call itself. The
// CallBuilder.Call method from the returned CallBuilder instance can be used
// to do so.
func (c *CommonRPCInterface) RPC(methodName string) *CallBuilder {
	return c.XMLRPC.CallBuilder().Name(methodName).NamePrefix(c.XMLRPCMethodNamePrefix)
}

// Host returns the host and port of the device this RPC sends requests to.
func (c *CommonRPCInterface) Host() string {
	return fmt.Sprintf("%s:%d", c.XMLRPC.host, c.XMLRPC.port)
}

// CallBuilder is a utility for building and executing XMLRPC requests easily.
// This builder allows the user to specify the different elements of an RPC
// call as needed in a chain. It also provides a few convenience methods for
// making calls that expect a return of basic types.
type CallBuilder struct {
	xmlrpc           *XMLRpc
	methodNamePrefix string
	methodName       string
	methodArgs       []interface{}
	methodReturns    []interface{}
}

// NewCallBuilder creates a new CallBuilder instance.
func NewCallBuilder(xmlrpc *XMLRpc) *CallBuilder {
	return &CallBuilder{
		xmlrpc: xmlrpc,
	}
}

// Name sets the name of the method that will be called. This will be appended
// to any value passed to NamePrefix during Call.
func (b *CallBuilder) Name(methodName string) *CallBuilder {
	b.methodName = methodName
	return b
}

// NamePrefix sets the prefix of the name of the method that will be called.
// This will be prepended to method name set by Name during Call.
func (b *CallBuilder) NamePrefix(methodNamePrefix string) *CallBuilder {
	b.methodNamePrefix = methodNamePrefix
	return b
}

// Args sets the method arguments, in order, that will be sent with
// the XMLRPC call.
// Each passed argument must be of a data type that can be marshalled for an
// XMLRPC call by XMLRpc.
func (b *CallBuilder) Args(methodArgs ...interface{}) *CallBuilder {
	b.methodArgs = methodArgs
	return b
}

// Returns sets the return pointers, in order, that will be used to save the
// return values of the XMLRPC call. Once Call is completed successfully, these
// objects' values will be updated.
//
// Each passed return pointer must point to a data type that can be unmarshalled
// from an XMLRPC call response by XMLRpc.
func (b *CallBuilder) Returns(returnPointers ...interface{}) *CallBuilder {
	b.methodReturns = returnPointers
	return b
}

// Call completes the XMLRPC call build process by using what was set in other
// methods to execute the XMLRPC request with XMLRpc.
//
// Note that this only returns an error if the request fails, it does not return
// the return values of the RPC call. To retrieve the return values of the RPC
// call, call Returns prior to Call.
func (b *CallBuilder) Call(ctx context.Context) error {
	methodName := b.methodName
	if methodName == "" {
		return errors.New("method name must be set with the Name builder function")
	}
	if b.methodNamePrefix != "" {
		methodName = b.methodNamePrefix + methodName
	}
	if b.methodArgs == nil {
		b.methodArgs = []interface{}{}
	}
	if b.methodReturns == nil {
		b.methodReturns = []interface{}{}
	}
	if err := b.xmlrpc.Run(ctx, NewCall(methodName, b.methodArgs...), b.methodReturns...); err != nil {
		return errors.Wrapf(err, "failed XMLRPC call to method %q", methodName)
	}
	return nil
}

// CallForBool is a convenience method for calling an RPC method that returns
// a single bool. The result of the RPC method call and Call is returned.
func (b *CallBuilder) CallForBool(ctx context.Context) (bool, error) {
	var result bool
	err := b.Returns(&result).Call(ctx)
	if err != nil {
		return false, err
	}
	return result, nil
}

// CallForString is a convenience method for calling an RPC method that returns
// a single string. The result of the RPC method call and Call is returned.
func (b *CallBuilder) CallForString(ctx context.Context) (string, error) {
	var result string
	err := b.Returns(&result).Call(ctx)
	if err != nil {
		return "", err
	}
	return result, nil
}

// CallForInt is a convenience method for calling an RPC method that returns
// a single int. The result of the RPC method call and Call is returned.
func (b *CallBuilder) CallForInt(ctx context.Context) (int, error) {
	var result int
	err := b.Returns(&result).Call(ctx)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// CallForInts is a convenience method for calling an RPC method that returns
// a single int array. The result of the RPC method call and Call is returned.
func (b *CallBuilder) CallForInts(ctx context.Context) ([]int, error) {
	var result []int
	err := b.Returns(&result).Call(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// CallForBytes is a convenience method for calling an RPC method that returns
// a single byte array. The result of the RPC method call and Call is returned.
func (b *CallBuilder) CallForBytes(ctx context.Context) ([]byte, error) {
	var result []byte
	err := b.Returns(&result).Call(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}
