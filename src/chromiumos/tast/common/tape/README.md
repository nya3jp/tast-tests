# Using TAPE for Generic Accounts

In order to use TAPE for generic accounts, the following steps must be
performed:

1. Determine a Pool ID that the accounts will be grouped under. This can be
   based on the game, or application name (i.e. Roblox).
2. Get a list of accounts (with usernames, and passwords) that should be leased.
3. Add the accounts to the tast-tests-private/vars folder using the list format,
   i.e.

```yaml
test.<pool_id>Creds: |-
  username1:password1
  username2:password2
  ...
```

4. Import the accounts into the TAPE Google Cloud project under the
   generic_account Kind in Datastore. Make sure to use the <pool_id> that was
   defined in (1).
5. Register a new Remote fixture in the `RemoteFixtures` function of this
   package. The fixture should be named `Remote<pool_id>LeasedAccountFixture`.
   Note that the lease time must be enough to account for all tests that require
   the credentials. I.e. if there are two tests with timeouts of 15 minutes, and
   30 minutes, the time provided in the remote fixture should be 45 minutes.
6. In the tests that require the remote fixtures, update their fixture to define
   a `Parent:` which matches the name of the registered fixture from (5).
7. In the tests which require the credentials, load the defined variables from (
   3) and call tape.LeasedAccount. This will find the leased account in the list
   of provided credentials. The returned username, and password and guaranteed
   to be only in use by the current test.
