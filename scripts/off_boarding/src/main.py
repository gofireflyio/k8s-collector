from api_client.firefly_api_client import FireflyApiClient
from requests import HTTPError, ConnectionError
import logging
import os


def main():
    access_key = os.getenv("ACCESS_KEY")
    if access_key is None:
        logging.error("ACCESS_KEY is not set")

    secret_key = os.getenv("SECRET_KEY")
    if secret_key is None:
        logging.error("SECRET_KEY is not set")

    cluster_id = os.getenv("CLUSTER_ID")
    if cluster_id is None:
        logging.error("CLUSTER_ID is not set")

    app_api_url = os.getenv("APP_API_URL", "")
    if app_api_url == "":
        app_api_url = "https://prodapi.gofirefly.io"

    try:
        api_client = FireflyApiClient(base_url=app_api_url, access_key=access_key, secret_key=secret_key)
    except HTTPError as e:
        logging.error(f"Failed to create firefly api client: {e}. Exiting...")
        exit(0)

    try:
        api_client.delete_k8s_integration(cluster_id=cluster_id)
    except HTTPError as e:
        logging.error(f"Failed to delete kubernetes integration: {e}. Exiting...")
        exit(0)

    logging.info(f"Kubernetes Integration {cluster_id} successfully deleted")


if __name__ == '__main__':
    main()

