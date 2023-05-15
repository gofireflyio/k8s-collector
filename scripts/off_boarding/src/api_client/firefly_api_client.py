from requests import Session, auth, HTTPError, ConnectionError, post
from retry import retry
import json
import logging


class FireflyAuth(auth.AuthBase):
    """Authentication Mechanism for FireFlyApiClient

    Args:
        requests (_type_): _description_
    """
    def __init__(self, base_url: str, access_key: str, secret_key: str):
        self._base_url = base_url
        self._access_key = access_key
        self._secret_key = secret_key

        self._login_firefly()

    def __call__(self, request):
        """Sets auth headers
        """
        request.headers['Authorization'] = self._bearer

        return request

    @retry(exceptions=(HTTPError, ConnectionError), tries=3, delay=5)
    def _login_firefly(self) -> str:
        """Internal Method to receive valid access token.

        Raises:
            requests.HTTPError: _description_
        """
        logging.info("Logging into Firefly")
        response = post(f'{self._base_url}/api/account/access_keys/login', {'accessKey': self._access_key, 'secretKey': self._secret_key})
        response.raise_for_status()

        token = json.loads(response.text)['access_token']
        self._bearer = f'Bearer {token}'


class FireflyApiClient:
    def __init__(self, base_url: str, access_key: str, secret_key: str) -> None:
        """
        Args:
            base_url (str): base URL for reaching Firefly API
            access_key (str): unique access key issued by Firefly
            secret_key (str): unique secret key issued by Firefly
        """
        self.session = Session()
        self.session.auth = FireflyAuth(base_url=base_url, access_key=access_key, secret_key=secret_key)

        self._base_url = base_url

    def delete_k8s_integration(self, cluster_id: str) -> None:
        """Deletes Kubernetes Integration from Firefly

        Args:
            cluster_id (str): unique
        """
        logging.info(f"Deleting Kubernetes Integration for Cluster {cluster_id}")
        response = self.session.delete(f'{self._base_url}/api/integrations/k8s/{cluster_id}')
        if response.status_code == 404:
            logging.info(f"Kubernetes Integration for Cluster {cluster_id} not found")
            return
        response.raise_for_status()


