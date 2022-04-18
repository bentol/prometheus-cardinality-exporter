package cardinality

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"

	"github.com/prometheus/client_golang/prometheus"

	logging "github.com/sirupsen/logrus"
)

var log = logging.WithFields(logging.Fields{})

// PrometheusClient interface for mock
type PrometheusClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// PrometheusGaugeVec interface for mock
type PrometheusGaugeVec interface {
	GetMetricWith(labels prometheus.Labels) (prometheus.Gauge, error)
	Delete(labels prometheus.Labels) bool
	Collect(ch chan<- prometheus.Metric)
	Describe(ch chan<- *prometheus.Desc)
}

// PrometheusCardinalityMetric used to apply methods to a PrometheusGaugeVec (updateMetric)
type PrometheusCardinalityMetric struct {
	GaugeVec PrometheusGaugeVec
}

// Struct for retaining a single label value pair
type labelValuePair struct {
	Label string `json:"name"`
	Value uint64 `json:"value"`
}

type prometheusLabelValuePair struct {
	Labels prometheus.Labels
	Value  uint64
}

// TSDBData contains the metric updates
type TSDBData struct {
	SeriesCountByMetricName     [10]labelValuePair `json:"seriesCountByMetricName"`
	LabelValueCountByLabelName  [10]labelValuePair `json:"labelValueCountByLabelName"`
	MemoryInBytesByLabelName    [10]labelValuePair `json:"memoryInBytesByLabelName"`
	SeriesCountByLabelValuePair [10]labelValuePair `json:"seriesCountByLabelValuePair"`
}

// TSDBStatus : a struct to hold data returned by the Prometheus API call
type TSDBStatus struct {
	Status string   `json:"status"`
	Data   TSDBData `json:"data"`
}

// TrackedLabelNames : a struct to keep track of which metrics we are currently tracking
type TrackedLabelNames struct {
	SeriesCountByMetricNameLabels                 [10]string
	LabelValueCountByLabelNameLabels              [10]string
	MemoryInBytesByLabelNameLabels                [10]string
	SeriesCountByLabelValuePairLabels             [10]string
	SeriesCountByMetricNamePerLabelLabels         map[string][10]prometheus.Labels
	LabelValueCountByLabelNamePerMetricNameLabels map[string][10]prometheus.Labels
}

// PrometheusCardinalityInstance stores all that is required to know about  prometheus instance
// inc. it's name, it's address, the latest api call results, and the labels currently being tracked
type PrometheusCardinalityInstance struct {
	Namespace           string
	InstanceName        string
	InstanceAddress     string
	ShardedInstanceName string
	AuthValue           string
	LatestTSDBStatus    TSDBStatus
	TrackedLabels       TrackedLabelNames
}

func (promInstance *PrometheusCardinalityInstance) fetchTSDBStatus(prometheusClient PrometheusClient, query url.Values) ([]byte, error) {
	// Create a GET request to the Prometheus API
	apiURL := promInstance.InstanceAddress + "/api/v1/status/tsdb?" + query.Encode()
	request, err := http.NewRequest("GET", apiURL, nil)

	if promInstance.AuthValue != "" {
		request.Header.Add("Authorization", promInstance.AuthValue)
	}

	if err != nil {
		return nil, fmt.Errorf("Cannot create GET request to %v: %v", apiURL, err)
	}

	// Perform GET request
	res, err := prometheusClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Can't connect to %v: %v ", apiURL, err)
	}
	defer res.Body.Close()

	// Check the response and either log it, if 2xx, or return an error
	responseStatusLog := fmt.Sprintf("Request to %s returned status %s.", apiURL, res.Status)
	statusOK := res.StatusCode >= 200 && res.StatusCode < 300
	if !statusOK {
		return nil, errors.New(responseStatusLog)
	}
	log.Debug(responseStatusLog)

	// Read the body of the response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Can't read from socket: %v", err)
	}
	return body, nil
}

// FetchTSDBStatus saves tracked TSDB status metrics in the struct pointed to by the "data" parameter
func (promInstance *PrometheusCardinalityInstance) PreFetchTSDBStatus(prometheusClient PrometheusClient) error {
	body, err := promInstance.fetchTSDBStatus(prometheusClient, nil)
	if err != nil {
		return err
	}

	// Parse the JSON response body into a struct
	err = json.Unmarshal(body, &promInstance.LatestTSDBStatus)
	if err != nil {
		return fmt.Errorf("Can't parse json: %v", err)
	}
	return nil
}

// ExposeTSDBStatus expose TSDB status to /metrics
func (promInstance *PrometheusCardinalityInstance) ExposeTSDBStatus(seriesCountByMetricNameGauge, labelValueCountByLabelNameGauge, memoryInBytesByLabelNameGauge, seriesCountByLabelValuePairGauge *PrometheusCardinalityMetric) (err error) {

	promInstance.TrackedLabels.SeriesCountByMetricNameLabels, err = seriesCountByMetricNameGauge.updateMetric(promInstance.LatestTSDBStatus.Data.SeriesCountByMetricName, promInstance.TrackedLabels.SeriesCountByMetricNameLabels, promInstance.InstanceName, promInstance.ShardedInstanceName, promInstance.Namespace, "metric")
	if err != nil {
		return err
	}
	promInstance.TrackedLabels.LabelValueCountByLabelNameLabels, err = labelValueCountByLabelNameGauge.updateMetric(promInstance.LatestTSDBStatus.Data.LabelValueCountByLabelName, promInstance.TrackedLabels.LabelValueCountByLabelNameLabels, promInstance.InstanceName, promInstance.ShardedInstanceName, promInstance.Namespace, "label")
	if err != nil {
		return err
	}
	promInstance.TrackedLabels.MemoryInBytesByLabelNameLabels, err = memoryInBytesByLabelNameGauge.updateMetric(promInstance.LatestTSDBStatus.Data.MemoryInBytesByLabelName, promInstance.TrackedLabels.MemoryInBytesByLabelNameLabels, promInstance.InstanceName, promInstance.ShardedInstanceName, promInstance.Namespace, "label")
	if err != nil {
		return err
	}
	promInstance.TrackedLabels.SeriesCountByLabelValuePairLabels, err = seriesCountByLabelValuePairGauge.updateMetric(promInstance.LatestTSDBStatus.Data.SeriesCountByLabelValuePair, promInstance.TrackedLabels.SeriesCountByLabelValuePairLabels, promInstance.InstanceName, promInstance.ShardedInstanceName, promInstance.Namespace, "label_pair")
	if err != nil {
		return err
	}

	return nil

}
func (promInstance *PrometheusCardinalityInstance) ExposeSeriesCountByMetricsNamePerLabels(metrics *PrometheusCardinalityMetric) (err error) {
	for _, value := range promInstance.LatestTSDBStatus.Data.LabelValueCountByLabelName {
		fmt.Println("ExposeSeriesCountByMetricsNamePerLabels", value.Label)
		if value.Label == "" {
			continue
		}

		if err := promInstance.exposeSeriesCountByMetricsNamePerLabel(value.Label, metrics); err != nil {
			fmt.Println(err)
			return err
		}
	}
	return nil
}

func (promInstance *PrometheusCardinalityInstance) exposeSeriesCountByMetricsNamePerLabel(label string, metrics *PrometheusCardinalityMetric) (err error) {
	prometheusClient := &http.Client{}
	q, _ := url.ParseQuery(fmt.Sprintf(`match[]={%s!=""}`, label))
	body, err := promInstance.fetchTSDBStatus(prometheusClient, q)
	tsdbStatus := &TSDBStatus{}
	err = json.Unmarshal(body, tsdbStatus)
	if err != nil {
		return fmt.Errorf("Can't parse json: %v", err)
	}

	labelsValuePairs := [10]prometheusLabelValuePair{}
	for idx, pair := range tsdbStatus.Data.SeriesCountByMetricName {
		if len(pair.Label) > 0 {
			labels := prometheus.Labels{
				"metric": pair.Label,
				"label":  label,
			}
			labelsValuePairs[idx] = prometheusLabelValuePair{labels, pair.Value}
		}
	}

	if promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels == nil {
		promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels = map[string][10]prometheus.Labels{}
	}
	if _, ok := promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels[label]; !ok {
		promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels[label] = [10]prometheus.Labels{}
	}
	promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels[label], err = metrics.updateMetricByPrometheusLabels(labelsValuePairs, promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels[label], promInstance.InstanceName, promInstance.ShardedInstanceName, promInstance.Namespace)
	if err != nil {
		return fmt.Errorf("Can't update metrics: %v", err)
	}
	return nil
}

func (promInstance *PrometheusCardinalityInstance) ExposeLabelCountByLabelNameNamePerMetricNames(metrics *PrometheusCardinalityMetric) (err error) {
	for _, value := range promInstance.LatestTSDBStatus.Data.SeriesCountByMetricName {
		fmt.Println("ExposeLabelCountByLabelNameNamePerMetricNames", value.Label)
		if value.Label == "" {
			continue
		}

		if err := promInstance.exposeLabelCountByLabelNameNamePerMetricName(value.Label, metrics); err != nil {
			fmt.Println(err)
			return err
		}
	}
	return nil
}

func (promInstance *PrometheusCardinalityInstance) exposeLabelCountByLabelNameNamePerMetricName(metricName string, metrics *PrometheusCardinalityMetric) (err error) {
	prometheusClient := &http.Client{}
	q, _ := url.ParseQuery(fmt.Sprintf(`match[]={__name__="%s"}`, metricName))
	body, err := promInstance.fetchTSDBStatus(prometheusClient, q)
	tsdbStatus := &TSDBStatus{}
	err = json.Unmarshal(body, tsdbStatus)
	if err != nil {
		return fmt.Errorf("Can't parse json: %v", err)
	}

	labelsValuePairs := [10]prometheusLabelValuePair{}
	for idx, pair := range tsdbStatus.Data.LabelValueCountByLabelName {
		if len(pair.Label) > 0 {
			labels := prometheus.Labels{
				"metric": metricName,
				"label":  pair.Label,
			}
			labelsValuePairs[idx] = prometheusLabelValuePair{labels, pair.Value}
		}
	}

	if promInstance.TrackedLabels.LabelValueCountByLabelNamePerMetricNameLabels == nil {
		promInstance.TrackedLabels.LabelValueCountByLabelNamePerMetricNameLabels = map[string][10]prometheus.Labels{}
	}
	if _, ok := promInstance.TrackedLabels.LabelValueCountByLabelNamePerMetricNameLabels[metricName]; !ok {
		promInstance.TrackedLabels.LabelValueCountByLabelNamePerMetricNameLabels[metricName] = [10]prometheus.Labels{}
	}
	promInstance.TrackedLabels.LabelValueCountByLabelNamePerMetricNameLabels[metricName], err = metrics.updateMetricByPrometheusLabels(labelsValuePairs, promInstance.TrackedLabels.SeriesCountByMetricNamePerLabelLabels[metricName], promInstance.InstanceName, promInstance.ShardedInstanceName, promInstance.Namespace)
	if err != nil {
		return fmt.Errorf("Can't update metrics: %v", err)
	}
	return nil
}

func (Metric *PrometheusCardinalityMetric) updateMetricByPrometheusLabels(newLabelsValues [10]prometheusLabelValuePair, trackedLabels [10]prometheus.Labels, prometheusInstance string, shardedInstance string, namespace string) (newTrackedLabels [10]prometheus.Labels, err error) {
	for idx, prometheusLabelValuePair := range newLabelsValues {
		prometheusLabels := prometheusLabelValuePair.Labels
		if len(prometheusLabels) == 0 {
			break
		}
		prometheusLabels["scraped_instance"] = prometheusInstance
		prometheusLabels["sharded_instance"] = shardedInstance
		prometheusLabels["instance_namespace"] = namespace
		metricGauge, err := Metric.GaugeVec.GetMetricWith(prometheusLabels)
		if err != nil {
			return trackedLabels, fmt.Errorf("Error updating metric with labels %v: %v", prometheusLabels, err)
		}
		metricGauge.Set(float64(prometheusLabelValuePair.Value))
		newTrackedLabels[idx] = prometheusLabelValuePair.Labels
	}

	for _, oldLabel := range trackedLabels {
		found := false
		for _, newLabelVP := range newLabelsValues {
			if reflect.DeepEqual(oldLabel, newLabelVP.Labels) {
				found = true
				break
			}
		}
		if !found && len(oldLabel) > 0 {
			oldLabel["scraped_instance"] = prometheusInstance
			oldLabel["sharded_instance"] = shardedInstance
			oldLabel["instance_namespace"] = namespace
			Metric.GaugeVec.Delete(oldLabel)
		}
	}

	return newTrackedLabels, nil
}

// Updates the given metric with new values and deletes ones which are no longer being reported
func (Metric *PrometheusCardinalityMetric) updateMetric(newLabelsValues [10]labelValuePair, trackedLabels [10]string, prometheusInstance string, shardedInstance string, namespace string, nameOfLabel string) (newTrackedLabels [10]string, err error) {

	for idx, labelValuePair := range newLabelsValues {
		if labelValuePair.Label == "" {
			break
		}
		metricGauge, err := Metric.GaugeVec.GetMetricWith(prometheus.Labels{nameOfLabel: labelValuePair.Label, "scraped_instance": prometheusInstance, "sharded_instance": shardedInstance, "instance_namespace": namespace})
		if err != nil {
			return trackedLabels, fmt.Errorf("Error updating metric with label name %v: %v", labelValuePair.Label, err)
		}
		metricGauge.Set(float64(labelValuePair.Value))
		newTrackedLabels[idx] = labelValuePair.Label
	}

	for _, oldLabel := range trackedLabels {
		found := false
		for _, newLabelVP := range newLabelsValues {
			if oldLabel == newLabelVP.Label {
				found = true
				break
			}
		}
		if !found && oldLabel != "" {
			Metric.GaugeVec.Delete(prometheus.Labels{nameOfLabel: oldLabel, "scraped_instance": prometheusInstance, "sharded_instance": shardedInstance, "instance_namespace": namespace})
		}
	}

	return newTrackedLabels, nil

}
