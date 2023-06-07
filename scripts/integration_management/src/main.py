from api_client.firefly_api_client import FireflyApiClient
from data_classes.workflowflags import WORKFLOWFLAGS, OFFBOARD, ONBOARD
from k8s_integrations import delete_k8s_integration, create_k8s_integration_idempotent
from requests import HTTPError
import logging
import os


def main():
    access_key = os.getenv("ACCESS_KEY")
    if access_key is None:
        logging.error("ACCESS_KEY is not set")
        exit(0)

    secret_key = os.getenv("SECRET_KEY")
    if secret_key is None:
        logging.error("SECRET_KEY is not set")
        exit(0)

    cluster_id = os.getenv("CLUSTER_ID")
    if cluster_id is None:
        logging.error("CLUSTER_ID is not set")
        exit(0)

    integration_flow = os.getenv("INTEGRATION_FLOW")
    if integration_flow is None:
        logging.error("INTEGRATION_FLOW is not set")
        exit(0)

    is_prod = os.getenv("IS_PROD", False)

    app_api_url = os.getenv("APP_API_URL", "")
    if app_api_url == "":
        app_api_url = "https://prodapi.gofirefly.io"

    try:
        api_client = FireflyApiClient(base_url=app_api_url, access_key= access_key, secret_key=secret_key)
    except (HTTPError, ConnectionError) as e:
        logging.error(f"Failed to create firefly api client: {e}. Exiting...")
        exit(0)

    integration_flow = os.getenv("INTEGRATION_FLOW")
    if integration_flow not in WORKFLOWFLAGS:
        logging.error(f"INTEGRATION_FLOW must be one of: {WORKFLOWFLAGS}")
        exit(0)

    if integration_flow == ONBOARD:
        create_k8s_integration_idempotent(api_client, cluster_id, access_key, is_prod)
    elif integration_flow == OFFBOARD:
        delete_k8s_integration(api_client, cluster_id)



if __name__ == '__main__':
    main()

