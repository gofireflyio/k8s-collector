from requests.exceptions import HTTPError
import logging

from api_client.firefly_api_client import FireflyApiClient


def delete_k8s_integration(api_client: FireflyApiClient, cluster_id: str) -> None:
    try:
        api_client.delete_k8s_integration(cluster_id=cluster_id)
    except HTTPError as e:
        logging.error(f"Failed to delete kubernetes integration: {e}. Exiting...")
        exit(0)

    logging.info(f"Kubernetes Integration {cluster_id} successfully deleted")


def create_k8s_integration_idempotent(api_client: FireflyApiClient, cluster_id: str, access_key: str, is_prod: bool) -> None:
    integration_exists = False
    try:
        integration_exists = api_client.does_k8s_integration_exist(cluster_id=cluster_id)
    except HTTPError as e:
        logging.error(f"Failed to check if kubernetes integration exists: {e}. Exiting...")
        exit(0)
    if integration_exists:
        logging.info(f"Kubernetes Integration for Cluster {cluster_id} already exists. Skipping...")
        return
    else:
        try:
            api_client.create_k8s_integration(cluster_id=cluster_id, access_key=access_key, is_prod=is_prod)
        except HTTPError as e:
            logging.error(f"Failed to create kubernetes integration: {e}. Exiting...")
            exit(0)

    logging.info(f"Kubernetes Integration for Cluster {cluster_id} successfully created")