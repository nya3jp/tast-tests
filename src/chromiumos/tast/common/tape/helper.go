// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"

	"chromiumos/tast/errors"
)

// ServiceAccountVar holds the name of the variable which stores the service account credentials for TAPE.
const ServiceAccountVar = "tape.service_account_key"

type clientOption struct {
	client    *client
	credsJSON []byte
}

// ClientOption provides options for getting a client for an account helper.
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

type genericAccountHelper struct {
	client   *client
	Accounts []*GenericAccount
}

// NewGenericAccountHelper leases a generic account, stores it in a genericAccountHelper struct and returns both. It requires either the
// WithClient or the WithCredsJSON ClientOption to assign a tape client to the genericAccountHelper.
func NewGenericAccountHelper(ctx context.Context, request *requestGenericAccountParams, opts ...ClientOption) (*genericAccountHelper, *GenericAccount, error) {

	client, err := getClient(ctx, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tape client")
	}

	helper := &genericAccountHelper{
		client: client,
	}

	account, err := helper.RequestAccount(ctx, request)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to request account")
	}
	return helper, account, nil
}

// RequestAccount leases a generic account, stores it in the genericAccountHelper and returns it.
func (ah *genericAccountHelper) RequestAccount(ctx context.Context, params *requestGenericAccountParams) (*GenericAccount, error) {
	account, err := ah.client.RequestGenericAccount(ctx, params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request owned test account")
	}

	ah.Accounts = append(ah.Accounts, account)

	return account, nil
}

// CleanUp releases all generic accounts that are stored in the genericAccountHelper.
func (ah *genericAccountHelper) CleanUp(ctx context.Context) error {
	var combinedErrors error
	for _, account := range ah.Accounts {
		err := ah.client.ReleaseGenericAccount(ctx, account)
		if err != nil {
			combinedErrors = errors.Wrap(combinedErrors, err.Error())
		}
	}

	if combinedErrors.Error() != "" {
		return errors.Wrap(combinedErrors, "failed to release some accounts")
	}

	return nil
}

type ownedTestAccountHelper struct {
	client   *client
	Accounts []*OwnedTestAccount
}

// NewOwnedTestAccountHelper leases an owned test account, stores it in an ownedTestAccountHelper struct and returns both. It requires
// either the WithClient or the WithCredsJSON ClientOption to assign a tape client to the ownedTestAccountHelper.
func NewOwnedTestAccountHelper(ctx context.Context, request *requestOwnedTestAccountParams, opts ...ClientOption) (*ownedTestAccountHelper, *OwnedTestAccount, error) {

	client, err := getClient(ctx, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get tape client")
	}

	helper := &ownedTestAccountHelper{
		client: client,
	}

	account, err := helper.RequestAccount(ctx, request)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to request account")
	}
	return helper, account, nil
}

// RequestAccount leases an owned test account, stores it in the ownedTestAccountHelper and returns it.
func (ah *ownedTestAccountHelper) RequestAccount(ctx context.Context, params *requestOwnedTestAccountParams) (*OwnedTestAccount, error) {
	account, err := ah.client.RequestOwnedTestAccount(ctx, params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request owned test account")
	}

	ah.Accounts = append(ah.Accounts, account)

	return account, nil
}

// CleanUp releases all owned test accounts that are stored in the ownedTestAccountHelper.
func (ah *ownedTestAccountHelper) CleanUp(ctx context.Context) error {
	var combinedErrors error
	for _, account := range ah.Accounts {
		err := ah.client.ReleaseOwnedTestAccount(ctx, account)
		if err != nil {
			combinedErrors = errors.Wrap(combinedErrors, err.Error())
		}
	}

	if combinedErrors.Error() != "" {
		return errors.Wrap(combinedErrors, "failed to release some accounts")
	}

	return nil
}
