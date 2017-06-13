import logging
import os

import pytest
import json
import base64
import common
from bravado.swagger_model import load_file
from bravado.client import SwaggerClient, RequestsClient
from requests.utils import parse_header_links
from urllib import parse as urlparse
import requests

class SwaggerApiClient:
    config = {
        'also_return_response': True,
        'validate_responses': True,
        'validate_requests': False,
        'validate_swagger_spec': False,
        'use_models': True,
    }

    log = logging.getLogger('client.SwaggerApiClient')
    spec_option = 'spec'

    def setup_swagger(self):
        self.http_client = RequestsClient()
        self.http_client.session.verify = False

        spec = pytest.config.getoption(self.spec_option)
        self.client = SwaggerClient.from_spec(load_file(spec),
                                              config=self.config,
                                              http_client=self.http_client)

        self.client.swagger_spec.api_url = "http://{}/api/{}/".format(pytest.config.getoption("host"),
                                                                      pytest.config.getoption("api"))

    def make_api_url(self, path):
        return os.path.join(self.client.swagger_spec.api_url,
                            path if not path.startswith("/") else path[1:])

class InternalClient(SwaggerApiClient):
    log = logging.getLogger('client.InternalClient')

    spec_option = 'internal_spec'

    def setup(self):
        self.setup_swagger()

    def delete_user(self, id, auth=None):
        return requests.delete(self.make_api_url('/users?id={}'.format(id)), headers=auth)

    def make_id_auth(self, user, tenant):
        jwt = common.make_id_jwt(user, tenant)
        return {"Authorization" : "Bearer " + jwt}

class ManagementClient(SwaggerApiClient):
    log = logging.getLogger('client.ManagementClient')

    spec_option = 'management_spec'

    # default user auth - single user, single tenant
    uauth = {"Authorization": "Bearer foobarbaz"}

    def setup(self):
        self.setup_swagger()

    def make_user_auth(self, user_id, tenant_id=None):
        """
            Prepare an almost-valid JWT auth header, suitable for consumption by deviceadm.
        """
        jwt = common.make_id_jwt(user_id, tenant_id)
        return {"Authorization": "Bearer " + jwt}


class ManagementClientSimple(ManagementClient):
    log = logging.getLogger('client.ManagementClientSimple')

    def __init__(self):
        self.setup_swagger()
