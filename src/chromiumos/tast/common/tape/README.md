# TAPE Security
Note that TAPE requires local OAUTH for running locally, and uses known service
accounts when running in the lab. Context for this can be found here:
https://bugs.chromium.org/p/chromium/issues/detail?id=1339418. This is all
handled by the TAPE logic but requires a few manual steps when running tests
that use the TAPE service.

## Generating a local refresh token
All of this must be done outside of chroot. After the refresh token is generated,
re-authentication should not be required. Note that parameters (denoted as
`{{PARAM_NAME}}`) should be retrieved from https://source.corp.google.com/chromeos_internal/src/platform/tast-tests-private/vars/tape.yaml.

1. In a browser, navigate to:
   1. https://accounts.google.com/o/oauth2/auth?client_id={{CLIENT_ID}}&response_type=code&scope=openid%20email&access_type=offline&redirect_uri=urn:ietf:wg:oauth:2.0:oob
2. Authenticate and copy the retrieved code. This will be referred to as {{AUTH_CODE}}
3. Run:
```shell
curl --verbose \
	      --data client_id={{CLIENT_ID}} \
	      --data client_secret={{CLIENT_SECRET}} \
	      --data code={{AUTH_CODE}} \
	      --data redirect_uri=urn:ietf:wg:oauth:2.0:oob \
	      --data grant_type=authorization_code \
	      https://oauth2.googleapis.com/token
```
4. Save the value of `refresh_token`. This will be used when running tests.

## Testing locally
1. Add `--var tape.local_refresh_token=<VALUE>` when running a test that uses TAPE.

## Notes
1. If you are unable to access TAPE, ask davidwelling@ or alexanderhartl@ to have your email address added to the list of authenticated users in IAP.