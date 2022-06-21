GREEN='\033[0;32m'
RED='\033[0;31m'
NO_COLOR='\033[0m'

FIREFLY_COLLECTOR_NAMESPACE="firefly"
HELM_RELEASE_NAME="infralight"

echo -e "${GREEN}validating jq is installed...${NO_COLOR}"
jq --version
if [[ $? -ne 0 ]]; then
  echo -e "${RED}jq is not installed. Please install it using brew/apt/etc.${NO_COLOR}"
  exit 1
fi

echo -e "${GREEN}getting helm values...${NO_COLOR}"
VALUES=`helm -n $FIREFLY_COLLECTOR_NAMESPACE -o json get values $HELM_RELEASE_NAME`
ACCESS_KEY=`echo $VALUES | jq -r '.accessKey'`
SECRET_KEY=`echo $VALUES | jq -r '.secretKey'`
CLUSTER_ID=`echo $VALUES | jq -r '.clusterId'`

echo -e "${GREEN}Uninstalling old helm release and repo${NO_COLOR}"
helm uninstall $HELM_RELEASE_NAME -n $FIREFLY_COLLECTOR_NAMESPACE && helm repo remove $HELM_RELEASE_NAME

echo -e "${GREEN}Installing new helm chart...${NO_COLOR}"
helm repo add firefly https://gofireflyio.github.io/k8s-collector
helm install firefly firefly/firefly-k8s-collector --set accessKey=$ACCESS_KEY --set secretKey=$SECRET_KEY --set clusterId=$CLUSTER_ID --set schedule="*/15 * * * *"  --namespace=firefly --create-namespace

echo -e "${GREEN}Finished updating helm${NO_COLOR}"
