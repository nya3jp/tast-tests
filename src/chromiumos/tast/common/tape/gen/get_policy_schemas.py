#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# Copyright 2021 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tool for getting and saving ChromePolicy API policy schemas from the TAPE
server.

Used to create policy_schemas.json.

Generating a new policy_schemas.json file:
  Running the script requires the client secret of the default service account
  of TAPE. If you don't have access to it contact cros-engprod-muc@google.com
  to regenerate policy_schemas.json for you.

  Otherwise run the script and follow the prompts. You will be requested to
  follow a link to generate an authorization code and paste it for use in the
  script.
  > ./get_policy_schemas.py --tape_client_secret <secret>
  Use a CL to check in the updated file.
"""

import argparse
import json
import os
import requests

# Default output filename and path.
OUTPUT_FILENAME = 'policy_schemas.json'
OUTPUT_FILEPATH = os.path.join(os.path.dirname(__file__), "..",
  OUTPUT_FILENAME)

# URLS
OAUTH_AUTH_URL = 'https://accounts.google.com/o/oauth2/v2/auth'
OAUTH_TOKEN_URL = 'https://oauth2.googleapis.com/token'
POLICY_SCHEMAS_URL = 'https://test-dot-tape-307412.ey.r.appspot.com/getPolicySchemas'

# TAPE
TAPE_CLIENT_ID = '770216225211-4fnjia8bqvte0btelrb3c1fcm7i2m6o6.apps.googleusercontent.com'
TAPE_IAP_CLIENT_ID = '770216225211-ihjn20dlehf94m9l4l5h0b0iilvd1vhc.apps.googleusercontent.com'

def raise_response_error(key, response, source):
  """Raise an error that a given key was not found in a response."""
  raise KeyError(f'Could not find {key} in response {response} from {source}')

def main():
  parser = argparse.ArgumentParser()
  parser.add_argument('--out', dest='out', default=OUTPUT_FILEPATH, type=str,
    help=('Optional filepath for the output. By default, '
    'use the path where the output is checked in.'))
  parser.add_argument('--tape_client_secret', dest='tape_client_secret',
    type=str, help= ('Client secret for the TAPE default service account'))

  args = parser.parse_args()
  if not args.tape_client_secret:
    print("Please provide the client secret of the default TAPE service "
      "account. If you don't have access to it please contact "
      "cros-engprod-muc@google.com to regenerate the policy_schemas.go file "
      "for you.")
    return

  # We want to retrieve the latest policySchemas from TAPE server.
  # First we need an authorization code.
  url = f'{OAUTH_AUTH_URL}?client_id={TAPE_CLIENT_ID}'
  url += '&response_type=code&scope=openid%20email&access_type=offline&redirect_uri=urn:ietf:wg:oauth:2.0:oob'

  print('Visit the following URL to get the authorization code and'
    'copy-paste it below.:\n' + url)
  authorization_code = input('Authorization code:')

  # Request a refresh_token with the authorization code.
  url = OAUTH_TOKEN_URL
  payload = {
    'client_id': TAPE_CLIENT_ID,
    'client_secret': args.tape_client_secret,
    'code': authorization_code,
    'redirect_uri': 'urn:ietf:wg:oauth:2.0:oob',
    'grant_type': 'authorization_code',
  }

  with requests.post(url, data=payload) as r:
    if 'refresh_token' not in r.json():
      raise_response_error('refresh_token', r.json(), OAUTH_TOKEN_URL)
    refresh_token = r.json()['refresh_token']

  # Use the refresh_token to retrieve the oauth2 token for identification
  # against the IAP of the TAPE server.
  payload = {
    'client_id': TAPE_CLIENT_ID,
    'client_secret': args.tape_client_secret,
    'refresh_token': refresh_token,
    'redirect_uri': 'urn:ietf:wg:oauth:2.0:oob',
    'grant_type': 'refresh_token',
    'audience': TAPE_IAP_CLIENT_ID,
  }

  with requests.post(url, data=payload) as r:
    if 'id_token' not in r.json():
      raise_response_error('id_token', r.json(), OAUTH_TOKEN_URL)
    id_token=r.json()['id_token']
  r.close()

  # Finally make a call to TAPE to retrieve all policy schemas.
  url = POLICY_SCHEMAS_URL
  token = 'Bearer ' + id_token
  headers = {'Authorization': token}
  with requests.get(url, headers=headers) as r:
    if 'policySchemas' not in r.json():
      raise_response_error('policySchemas', r.json(), POLICY_SCHEMAS_URL)
    policy_schemas = (r.json()['policySchemas'])

  # Write policy schemas to the file.
  with open(args.out, 'w') as fh:
    fh.write('[')
    for policy_schema in policy_schemas[:-1]:
      fh.write(json.dumps(policy_schema, indent=2))
      fh.write(',')
    fh.write(json.dumps(policy_schemas[-1], indent=2))
    fh.write(']')

if __name__ == '__main__':
  main()

