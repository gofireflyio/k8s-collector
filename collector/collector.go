package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofireflyio/k8s-collector/collector/config"
	"github.com/gofireflyio/k8s-collector/collector/filter"
	"github.com/gofireflyio/k8s-collector/collector/k8stree"
	"github.com/ido50/requests"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"gopkg.in/mgo.v2/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	MaxItemSize = 1024 * 1500
)

var (
	TooEarlyError            = errors.New("server is unavailable for collecting kubernetes assets now")
	FetchingIsDisabled       = errors.New("cannot proceed with asset collection due to a suspension of the integration.\nPlease contact the Firefly support in order to restore your integration.")
	ClusterUniqueIdDuplicate = errors.New("cannot proceed with asset collection due to a reuse of another cluster configuration.\nPlease contact the Firefly support in order to repair your integration.")
)

// DataCollector is an interface for objects that collect data from K8s-related
// components such as the Kubernetes API Server or Helm
type DataCollector interface {
	// Source is a method that returns a unique name for the collector source
	Source() string

	// Run executes the data collector. The configuration object is always passed
	// and is never empty or nil. Every collector must return a name for the
	// key under which the data will be sent to the Infralight App Server, the
	// data itself (which is a list of arbitrary objects), and an optional error.
	Run(context.Context, *config.Config) (
		keyName string,
		data []interface{},
		err error,
	)
}

// Collector is an execution-scoped object encapsulating the entire collection
// process.
type Collector struct {
	// the JWT access token used to authenticate with the Infralight App server.
	// this is automatically generated
	accessToken string

	// the unique identifier of the cluster we're collecting data from (must be
	// provided externally)
	clusterID string

	integrationId string

	// Cluster configuration
	clusterConfig *rest.Config

	// the collector's configuration
	conf *config.Config

	log            *zerolog.Logger
	client         *requests.HTTPClient
	dataCollectors []DataCollector
	dataFilters    []filter.DataFilter
}

var clusterIDRegex = regexp.MustCompile(`^[a-z0-9-_]+$`)

// New creates a new instance of the Collector struct. A Kubernetes cluster ID
// must be provided, together with a configuration object and a list of objects
// implementing the DataCollector interface.
//
// The cluster ID is a string of alphanumeric characters, dashes and underscores,
// of any length. Spaces are not allowed.
//
// A configuration object must be provided.
func New(
	clusterID string,
	clusterConfig *rest.Config,
	conf *config.Config,
	dataCollectors ...DataCollector,
) *Collector {
	if conf == nil {
		panic("Configuration object must be provided")
	}

	return &Collector{
		conf:           conf,
		log:            conf.Log,
		clusterConfig:  clusterConfig,
		clusterID:      clusterID,
		dataCollectors: dataCollectors,
		dataFilters:    filter.All,
	}
}

// Run executes the collector. The process includes authentication with the
// Infralight App Server, execution of all data collectors, and sending of the
// data to the App Server for storage.
func (f *Collector) Run(ctx context.Context) (err error) {
	// verify cluster ID is valid
	if !clusterIDRegex.MatchString(f.clusterID) {
		return fmt.Errorf("invalid cluster ID, must match %s", clusterIDRegex)
	}

	f.log.Info().Str("Firefly Login Endpoint", f.conf.LoginEndpoint).Str("Firefly Endpoint", f.conf.Endpoint).Msg("Starting")

	// authenticate with the Infralight API
	if f.conf.DryRun {
		log.Info().Msg("Skipping authentication due to dry-run")
	} else {
		err = f.authenticate()
		if err != nil {
			return fmt.Errorf("failed authenticating with Infralight API: %w", err)
		}

		f.log.Info().Msg("Authenticated to Infralight App Server successfully")
	}

	var uniqueClusterId, fetchingId, integrationId string
	var sendTrees bool

	if f.conf.DryRun {
		uniqueClusterId = "dry-run-cluster-id"
		fetchingId = "dry-run-fetching-id"
	} else {
		uniqueClusterId, err = f.getUniqueClusterId(ctx)
		if err != nil {
			return fmt.Errorf("failed finding Kubernetes unique cluster ID: %w", err)
		}

		fetchingId, integrationId, sendTrees, err = f.startNewFetching(uniqueClusterId)
		if err != nil {
			if errors.Is(err, TooEarlyError) {
				f.log.Info().Msgf("Skipping this collection cycle, due to a remote error, error: %s", err.Error())
				return nil
			} else if errors.Is(err, FetchingIsDisabled) {
				f.log.Info().Msgf("%s", err.Error())
				return nil
			} else if errors.Is(err, ClusterUniqueIdDuplicate) {
				f.log.Info().Msgf("%s", err.Error())
				return nil
			}
			return fmt.Errorf("failed starting new fetching with Infralight API: %w", err)
		}
	}

	f.integrationId = integrationId

	log := f.log.With().
		Str("fetchingId", fetchingId).
		Str("integrationId", integrationId).
		Str("uniqueClusterId", uniqueClusterId).
		Logger()

	log.Info().Msg("Starting new fetching process")

	fullData := make(map[string][]interface{}, len(f.dataCollectors))

	log.Debug().Int("amount", len(f.dataCollectors)).Msg("Running Kubernetes collectors")

	for _, dc := range f.dataCollectors {
		keyName, data, err := dc.Run(ctx, f.conf)
		if err != nil {
			if keyName == "helm_releases" {
				log.Warn().Err(err).Msg("Failed fetching helm releases")
				fullData[keyName] = data
				continue
			}
			return fmt.Errorf("%s collector failed: %w", dc.Source(), err)
		}

		fullData[keyName] = data
	}

	for _, filter := range f.dataFilters {
		log.Debug().Msg("Running filter")
		err := filter(ctx, fullData)
		if err != nil {
			log.Warn().Err(err).Msg("Filter failed")
			continue
		}
	}

	if f.conf.DryRun {
		enc := json.NewEncoder(os.Stdout)
		err = enc.Encode(fullData)
		if err != nil {
			return fmt.Errorf("failed encoding collected data: %w", err)
		}

		return nil
	}

	log.Debug().Msg("Sending data to Infralight App Server")

	err = f.sendHelmReleases(fetchingId, fullData["helm_releases"], fullData["k8s_types"])
	if err != nil {
		return fmt.Errorf("failed sending releases to Infralight: %w", err)
	}

	if sendTrees {
		k8sTree, err := k8stree.GetK8sTree(fullData["k8s_objects"])
		if err != nil {
			return fmt.Errorf("failed getting k8s objects tree: %w", err)
		}

		err = f.sendK8sTree(fetchingId, k8sTree)
		if err != nil {
			return fmt.Errorf("failed sending k8s objects tree to Infralight: %w", err)
		}
	}

	err = f.sendK8sObjects(fetchingId, fullData["k8s_objects"])
	if err != nil {
		return fmt.Errorf("failed sending objects to Infralight: %w", err)
	}

	return nil
}

func (f *Collector) authenticate() (err error) {
	var credentials struct {
		Token     string `json:"access_token"`
		ExpiresIn int64  `json:"expires_in"`
		Type      string `json:"token_type"`
	}

	err = requests.NewClient(f.conf.LoginEndpoint).
		NewRequest("POST", "/account/access_keys/login").
		JSONBody(map[string]interface{}{
			"accessKey": f.conf.AccessKey,
			"secretKey": f.conf.SecretKey,
		}).
		Into(&credentials).
		Run()
	if err != nil {
		return err
	}

	f.client = requests.NewClient(f.conf.Endpoint).
		Timeout(f.conf.PageTimeoutDuration).
		RetryLimit(uint8(f.conf.MongoMaxRetries)).
		Header("Authorization", fmt.Sprintf("Bearer %s", credentials.Token)).
		CompressWith(requests.CompressionAlgorithmGzip).
		ErrorHandler(func(httpStatus int, contentType string, body io.Reader) error {
			if httpStatus == http.StatusTooEarly {
				return TooEarlyError
			}
			content, err := io.ReadAll(body)
			if err != nil {
				return fmt.Errorf("server returned unexpected status %d", httpStatus)
			}

			return fmt.Errorf("server returned %d: %q", httpStatus, content)
		})

	return nil
}

func (f *Collector) getUniqueClusterId(ctx context.Context) (clusterId string, err error) {
	kubeApi, err := kubernetes.NewForConfig(f.clusterConfig)
	if err != nil {
		return clusterId, fmt.Errorf("Failed creating Kubernetes Api object: %w", err)
	}

	kubeSystemNs, err := kubeApi.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return clusterId, fmt.Errorf("Failed finding `kube-system` Kubernetes namespace: %w", err)
	}

	return string(kubeSystemNs.GetObjectMeta().GetUID()), nil
}

func (f *Collector) startNewFetching(clusterUniqueId string) (fetchingId, integrationId string, sendTrees bool, err error) {
	fetchingId = bson.NewObjectId().Hex()
	var respoonse string
	req := f.client.
		NewRequest("GET", fmt.Sprintf("/integrations/k8s/%s/fetching", f.clusterID)).
		QueryParam("clusterUniqueId", clusterUniqueId).
		QueryParam("fetchingId", fetchingId).
		QueryParam("getIntegrationId", "true").
		ExpectedStatus(http.StatusOK).
		Into(&respoonse).
		ErrorHandler(func(httpStatus int, contentType string, body io.Reader) error {
			if httpStatus == http.StatusTooEarly {
				return TooEarlyError
			}
			if httpStatus == http.StatusNoContent {
				// old version returns 204
				return nil
			}
			if httpStatus == http.StatusPaymentRequired {
				// This kubernetes integration is suspended
				return FetchingIsDisabled
			} else if httpStatus == http.StatusConflict {
				// This kubernetes integration is suspended
				return ClusterUniqueIdDuplicate
			}
			content, err := io.ReadAll(body)
			if err != nil {
				return fmt.Errorf("server returned unexpected status %d", httpStatus)
			}

			return fmt.Errorf("server returned %d: %q", httpStatus, content)
		})
	if f.conf.OverrideUniqueClusterId {
		req.QueryParam("overrideUniqueClusterId", "1")
	}
	err = req.Run()

	responseSpllited := strings.Split(respoonse, ",")
	sendTrees = true
	if len(responseSpllited) > 1 {
		if value, err := strconv.ParseBool(responseSpllited[1]); err == nil {
			sendTrees = value
		}
	}

	return fetchingId, responseSpllited[0], sendTrees, err
}

func (f *Collector) send(data map[string]interface{}) error {
	f.conf.Log.Debug().
		Interface("data", data).
		Msg("Sending collected data to Infralight")

	return f.client.
		NewRequest("POST", fmt.Sprintf("/integrations/k8s/%s/fetching", f.clusterID)).
		ExpectedStatus(http.StatusNoContent).
		JSONBody(data).
		Run()
}

func (f *Collector) sendK8sObjects(fetchingId string, data []interface{}) error {
	if len(data) == 0 {
		f.conf.Log.Warn().
			Str("FetchingId", fetchingId).
			Msg("No k8s objects to send to Infralight")
		return nil
	}
	f.conf.Log.Debug().
		Int("TotalObjects", len(data)).
		Msg("Sending collected data to Infralight")

	totalBytes := 0
	var chunks [][]interface{}
	var objects []interface{}
	for idx, obj := range data {
		bytes, err := json.Marshal(obj)
		if err != nil {
			f.conf.Log.Err(err).
				Msg("failed to send resource")
		} else if len(bytes) > MaxItemSize {
			f.conf.Log.Warn().
				Msg("skipping massive resource")
		} else {
			totalBytes += len(bytes)
			objects = append(objects, obj)
		}
		if totalBytes > f.conf.PageSize*1000 || idx == len(data)-1 {
			chunks = append(chunks, objects)
			objects = []interface{}{}
			totalBytes = 0
		}
	}

	concurrentGoroutines := make(chan struct{}, f.conf.MaxGoRoutines)
	g, _ := errgroup.WithContext(context.Background())
	for _, chunkObjects := range chunks {
		concurrentGoroutines <- struct{}{}

		routineObjects := chunkObjects
		g.Go(func() error {
			defer func() {
				<-concurrentGoroutines
			}()
			body := make(map[string]interface{}, 2)
			body["fetchingId"] = fetchingId
			body["k8sObjects"] = routineObjects
			err := f.client.
				NewRequest(
					"POST",
					fmt.Sprintf("/integrations/k8s/%s/fetching/objects", f.clusterID),
				).
				QueryParam("integrationId", f.integrationId).
				ExpectedStatus(http.StatusNoContent).
				RetryLimit(uint8(f.conf.MongoMaxRetries)).
				JSONBody(body).
				Run()
			if err != nil {
				log.Err(err).Str("ClusterId", f.clusterID).Str("FetchingId", fetchingId).
					Int("ResourcesInPage", len(routineObjects)).
					Msg("Error sending resources to server")
				return err
			}
			log.Info().Str("ClusterId", f.clusterID).Str("FetchingId", fetchingId).
				Int("ResourcesInPage", len(routineObjects)).
				Msg("Sent k8s objects page successfully")
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	log.Info().
		Str("ClusterId", f.clusterID).
		Str("FetchingId", fetchingId).
		Str("IntegrationId", f.integrationId).
		Msg("Sending LOCK request")

	err := f.client.
		NewRequest("PATCH", fmt.Sprintf("/integrations/k8s/%s/fetching", f.clusterID)).
		QueryParam("integrationId", f.integrationId).
		ExpectedStatus(http.StatusNoContent).
		JSONBody(map[string]interface{}{
			"fetchingId": fetchingId,
			"clusterId":  f.clusterID,
		}).
		Run()
	if err != nil {
		log.Err(err).
			Str("ClusterId", f.clusterID).
			Str("FetchingId", fetchingId).
			Str("IntegrationId", f.integrationId).
			Msg("Error sending LOCK")
		return nil
	}
	log.Info().
		Str("ClusterId", f.clusterID).
		Str("FetchingId", fetchingId).
		Str("IntegrationId", f.integrationId).
		Msg("Sent LOCK successfully")
	return nil
}

func (f *Collector) sendHelmReleases(
	fetchingId string,
	data []interface{},
	types []interface{},
) error {
	if len(data) == 0 {
		f.conf.Log.Warn().
			Str("FetchingId", fetchingId).
			Msg("No helm releases to send to Infralight")
		return nil
	}
	f.conf.Log.Debug().
		Str("FetchingId", fetchingId).
		Int("HelmReleases", len(data)).
		Msg("Sending collected helm releases to Infralight")

	totalBytes := 0
	var chunks [][]interface{}
	var objects []interface{}

	for idx, obj := range data {
		bytes, _ := json.Marshal(obj)
		totalBytes += len(bytes)
		objects = append(objects, obj)

		if totalBytes > f.conf.PageSize*1000 || idx == len(data)-1 {
			chunks = append(chunks, objects)
			objects = []interface{}{}
			totalBytes = 0
		}
	}

	concurrentGoroutines := make(chan struct{}, f.conf.MongoMaxGoRoutines)
	g, _ := errgroup.WithContext(context.Background())
	for _, chunkObjects := range chunks {
		concurrentGoroutines <- struct{}{}

		routineObjects := chunkObjects
		g.Go(func() error {
			defer func() {
				<-concurrentGoroutines
			}()
			body := make(map[string]interface{}, 3)
			body["fetchingId"] = fetchingId
			body["helmReleases"] = routineObjects
			body["k8sTypes"] = types
			err := f.client.
				NewRequest("POST", fmt.Sprintf("/integrations/k8s/%s/fetching/helm", f.clusterID)).
				QueryParam("integrationId", f.integrationId).
				ExpectedStatus(http.StatusNoContent).
				JSONBody(body).
				Timeout(f.conf.PageTimeoutDuration).
				RetryLimit(uint8(f.conf.MongoMaxRetries)).
				Run()
			if err != nil {
				log.Err(err).Str("ClusterId", f.clusterID).Str("FetchingId", fetchingId).
					Int("ResourcesInPage", len(routineObjects)).
					Msg("Error sending resources to server")
				return err
			}
			log.Info().Str("ClusterId", f.clusterID).Str("FetchingId", fetchingId).
				Int("ResourcesInPage", len(routineObjects)).
				Msg("Sent helm releases page successfully")
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	log.Info().
		Str("FetchingId", fetchingId).
		Int("Resources", len(data)).
		Msg("Sent all helm releases successfully")
	return nil
}

func (f *Collector) sendK8sTree(fetchingId string, data []k8stree.ObjectsTree) error {
	if len(data) == 0 {
		f.conf.Log.Warn().
			Str("FetchingId", fetchingId).
			Msg("No k8s objects trees to send to Infralight")
		return nil
	}
	f.conf.Log.Debug().
		Str("FetchingId", fetchingId).
		Int("Trees", len(data)).
		Msg("Sending collected data to Infralight")

	totalBytes := 0
	var chunks [][]interface{}
	var objectsTrees []interface{}
	for idx, tree := range data {
		name := tree.Name
		tree.Name = ""
		bytes, err := json.Marshal(tree)
		if (tree.Children == nil || len(tree.Children) == 0) && tree.Kind != "Ingress" &&
			tree.Kind != "Provisioner" {
			f.conf.Log.Debug().
				Int("children", len(tree.Children)).
				Str("kind", tree.Kind).
				Str("name", name).
				Msg("skipping empty tree")
		} else if err != nil {
			f.conf.Log.Err(err).
				Int("children", len(tree.Children)).
				Str("kind", tree.Kind).
				Str("name", name).
				Msg("failed to send tree")
		} else if len(bytes) > MaxItemSize {
			f.conf.Log.Warn().
				Int("children", len(tree.Children)).
				Int("size", len(bytes)).
				Str("kind", tree.Kind).
				Str("name", name).
				Msg("skipping massive tree")
		} else {
			totalBytes += len(bytes)
			objectsTrees = append(objectsTrees, tree)
		}

		if totalBytes > f.conf.PageSize*1000 || idx == len(data)-1 {
			chunks = append(chunks, objectsTrees)
			objectsTrees = []interface{}{}
			totalBytes = 0
		}
	}

	concurrentGoroutines := make(chan struct{}, f.conf.MongoMaxGoRoutines)
	g, _ := errgroup.WithContext(context.Background())
	for _, chunkObjectsTrees := range chunks {
		concurrentGoroutines <- struct{}{}

		routineObjects := chunkObjectsTrees
		g.Go(func() error {
			defer func() {
				<-concurrentGoroutines
			}()
			body := make(map[string]interface{}, 2)
			body["fetchingId"] = fetchingId
			body["k8sTrees"] = routineObjects
			err := f.client.
				NewRequest("POST", fmt.Sprintf("/integrations/k8s/%s/fetching/tree", f.clusterID)).
				QueryParam("integrationId", f.integrationId).
				ExpectedStatus(http.StatusNoContent).
				JSONBody(body).
				Timeout(f.conf.PageTimeoutDuration).
				RetryLimit(uint8(f.conf.MongoMaxRetries)).
				Run()
			if err != nil {
				log.Err(err).Str("ClusterId", f.clusterID).Str("FetchingId", fetchingId).
					Int("ResourcesInPage", len(routineObjects)).
					Msg("Error sending resources to server")
				return err
			}
			log.Info().Str("ClusterId", f.clusterID).Str("FetchingId", fetchingId).
				Int("ResourcesInPage", len(routineObjects)).
				Msg("Sent k8s objects trees page successfully")
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	log.Info().
		Str("FetchingId", fetchingId).
		Int("Resources", len(data)).
		Msg("Sent k8s objects trees page successfully")
	return nil
}
