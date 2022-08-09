// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	ts "chromiumos/tast/services/cros/tape"
)

// ServiceAccountVar holds the name of the variable which stores the service account credentials for TAPE.
const ServiceAccountVar = "tape.service_account_key"

type clientOption struct {
	client      *client
	credsJSON   []byte
	requestOpts []RequestAccountOption
}

// ClientOption provides options for getting a client for an account manager.
type ClientOption func(*clientOption)

// WithCredsJSON provides the option to get a client from service account credentials.
func WithCredsJSON(jsonData []byte) ClientOption {
	return func(opt *clientOption) {
		opt.credsJSON = jsonData
	}
}

// WithClient provides the option to use an existing client.
func WithClient(client *client) ClientOption {
	return func(opt *clientOption) {
		opt.client = client
	}
}

func getClient(ctx context.Context, opts ...ClientOption) (*client, error) {
	// Copy over all options.
	options := clientOption{}
	for _, opt := range opts {
		opt(&options)
	}

	var client *client
	var err error
	if options.client != nil {
		client = options.client
	} else if len(options.credsJSON) > 0 {
		client, err = NewClient(ctx, options.credsJSON)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create tape client")
		}
	} else {
		return nil, errors.New("One of tape.client or credsJSON must be set")
	}
	return client, nil
}

// GenericAccountManager holds the client and the generic accounts data.
type GenericAccountManager struct {
	client   *client
	Accounts []*GenericAccount
}

// NewGenericAccountManager leases a generic account, stores it in a GenericAccountManager struct and returns both. It requires
// a credsJSON byte array with the credentials of a service account to create a tape client for the GenericAccountManager.
func NewGenericAccountManager(ctx context.Context, credsJSON []byte, opts ...RequestAccountOption) (*GenericAccountManager, *GenericAccount, error) {
	client, err := NewClient(ctx, credsJSON)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create tape client")
	}

	return NewGenericAccountManagerFromClient(ctx, client, opts...)
}

// NewGenericAccountManagerFromClient leases a generic account, stores it in a GenericAccountManager struct and returns both. It requires
// a tape client to assign it to the GenericAccountManager.
func NewGenericAccountManagerFromClient(ctx context.Context, client *client, opts ...RequestAccountOption) (*GenericAccountManager, *GenericAccount, error) {
	manager := &GenericAccountManager{
		client: client,
	}

	account, err := manager.RequestAccount(ctx, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to request account")
	}
	return manager, account, nil
}

// RequestAccount leases a generic account, stores it in the GenericAccountManager and returns it.
func (ah *GenericAccountManager) RequestAccount(ctx context.Context, opts ...RequestAccountOption) (*GenericAccount, error) {
	account, err := ah.client.RequestGenericAccount(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request owned test account")
	}

	ah.Accounts = append(ah.Accounts, account)

	return account, nil
}

// CleanUp releases all generic accounts that are stored in the GenericAccountManager.
func (ah *GenericAccountManager) CleanUp(ctx context.Context) error {
	var combinedErrors error
	for _, account := range ah.Accounts {
		err := ah.client.ReleaseGenericAccount(ctx, account)
		if err != nil {
			combinedErrors = errors.Wrap(combinedErrors, err.Error())
		}
	}

	if combinedErrors != nil {
		return errors.Wrap(combinedErrors, "failed to release some accounts")
	}

	return nil
}

// OwnedTestAccountManager holds the client and the Owned accounts data.
type OwnedTestAccountManager struct {
	client   *client
	Accounts []*OwnedTestAccount
}

// NewOwnedTestAccountManager leases an owned test account, stores it in an OwnedTestAccountManager struct and returns both. It requires
// a credsJSON byte array with the credentials of a service account to create a tape client for the OwnedTestAccountManager.
func NewOwnedTestAccountManager(ctx context.Context, credsJSON []byte, lock bool, opts ...RequestAccountOption) (*OwnedTestAccountManager, *OwnedTestAccount, error) {
	client, err := NewClient(ctx, credsJSON)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create tape client")
	}

	return NewOwnedTestAccountManagerFromClient(ctx, client, lock, opts...)
}

// NewOwnedTestAccountManagerFromClient leases an owned test account, stores it in an OwnedTestAccountManager struct and returns both.
// It requires a tape client to assign it to the OwnedTestAccountManager.
func NewOwnedTestAccountManagerFromClient(ctx context.Context, client *client, lock bool, opts ...RequestAccountOption) (*OwnedTestAccountManager, *OwnedTestAccount, error) {
	manager := &OwnedTestAccountManager{
		client: client,
	}

	account, err := manager.RequestAccount(ctx, lock, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to request account")
	}
	return manager, account, nil
}

// RequestAccount leases an owned test account, stores it in the OwnedTestAccountManager and returns it.
func (ah *OwnedTestAccountManager) RequestAccount(ctx context.Context, lock bool, opts ...RequestAccountOption) (*OwnedTestAccount, error) {
	account, err := ah.client.RequestOwnedTestAccount(ctx, lock, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request owned test account")
	}

	ah.Accounts = append(ah.Accounts, account)

	return account, nil
}

// CleanUp releases all owned test accounts that are stored in the OwnedTestAccountManager.
func (ah *OwnedTestAccountManager) CleanUp(ctx context.Context) error {
	var combinedErrors error
	for _, account := range ah.Accounts {
		err := ah.client.ReleaseOwnedTestAccount(ctx, account)
		if err != nil {
			combinedErrors = errors.Wrap(combinedErrors, err.Error())
		}
	}

	if combinedErrors != nil {
		return errors.Wrap(combinedErrors, "failed to release some accounts")
	}

	return nil
}

// DeprovisionHelper is a helper function to deprovision a device in a managed domain.
func (c *client) DeprovisionHelper(ctx context.Context, rpcClient *rpc.Client, customerID string) error {
	tapeService := ts.NewServiceClient(rpcClient.Conn)
	// Get the device ID of the DUT to deprovision it at the end of the test.
	res, err := tapeService.GetDeviceID(ctx, &ts.GetDeviceIDRequest{CustomerID: customerID})
	if err != nil {
		return errors.Wrap(err, "failed to get the deviceID")
	}
	if err = c.Deprovision(ctx, res.DeviceID, customerID); err != nil {
		return errors.Wrapf(err, "failed to deprovision device %s", res.DeviceID)
	}
	return nil
}
