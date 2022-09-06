package collector

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const METADATA_ENDPOINT = "http://169.254.169.254/latest/meta-data/"

type Collector struct {
	MetadataEndpoint          string
	SpotScrapeSuccessful      *prometheus.Desc
	SpotTerminationIndicator  *prometheus.Desc
	SpotTerminationTime       *prometheus.Desc
	ScheduledScrapeSuccessful *prometheus.Desc
	ScheduledActionIndicator  *prometheus.Desc
	ScheduledActionStartTime  *prometheus.Desc
	ScheduledActionEndTime    *prometheus.Desc
}

func NewCollector() *Collector {
	return &Collector{
		MetadataEndpoint: METADATA_ENDPOINT,
		SpotScrapeSuccessful: prometheus.NewDesc(
			"aws_instance_spot_metadata_service_available",
			"Spot metadata service available",
			[]string{"instance_id"},
			nil,
		),
		SpotTerminationIndicator: prometheus.NewDesc(
			"aws_instance_spot_termination_imminent",
			"Instance is about to be terminated",
			[]string{"instance_action", "instance_id"},
			nil,
		),
		SpotTerminationTime: prometheus.NewDesc(
			"aws_instance_spot_termination_in",
			"Instance will be terminated in",
			[]string{"instance_id"},
			nil,
		),
		ScheduledScrapeSuccessful: prometheus.NewDesc(
			"aws_instance_scheduled_metadata_service_available",
			"Scheduled actions metadata service available",
			[]string{"instance_id"},
			nil,
		),
		ScheduledActionIndicator: prometheus.NewDesc(
			"aws_instance_scheduled_action_iminent",
			"Instance count of scheduled actions",
			[]string{"instance_id"},
			nil,
		),
		ScheduledActionStartTime: prometheus.NewDesc(
			"aws_instance_scheduled_action_start_in",
			"Instance action will happen from",
			[]string{"instance_action", "instance_id"},
			nil,
		),
		ScheduledActionEndTime: prometheus.NewDesc(
			"aws_instance_scheduled_action_end_in",
			"Instance action will happen until",
			[]string{"instance_action", "instance_id"},
			nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.SpotScrapeSuccessful
	ch <- c.SpotTerminationIndicator
	ch <- c.SpotTerminationTime
	ch <- c.ScheduledScrapeSuccessful
	ch <- c.ScheduledActionIndicator
	ch <- c.ScheduledActionStartTime
	ch <- c.ScheduledActionEndTime
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	// Setup API client
	log.Debug("Acquiring AWS EC2 metadata API Token")
	client, err := newTokenizedHTTPClient()
	if err != nil {
		log.Errorf("Failed to initialize the AWS EC2 metadata API client: %s", err.Error())
		return
	}
	log.Debug("API client set-up with token header")

	// Get instance-id
	log.Debug("Getting instance-id")
	var instanceId string
	idResp, err := client.Get(c.MetadataEndpoint + "instance-id")
	if err != nil {
		log.Errorf("couldn't parse instance-id from metadata: %s", err.Error())
		return
	}
	if idResp.StatusCode == 404 {
		log.Errorf("couldn't parse instance-id from metadata: endpoint not found")
		return
	}
	defer idResp.Body.Close()
	body, _ := ioutil.ReadAll(idResp.Body)
	instanceId = string(body)
	log.Debugf("Got instance-id: %s", instanceId)

	// Collect metrics
	log.Debug("Retrieving metrics")
	for _, action := range c.scheduled_actions(client, instanceId) {
		ch <- action
	}
	for _, action := range c.spot_termination(client, instanceId) {
		ch <- action
	}
}

// scheduled_actions
//
// Retrieves scheduled maintenance info
func (c *Collector) scheduled_actions(client *http.Client, instanceId string) []prometheus.Metric {
	var ret []prometheus.Metric

	resp, err := client.Get(c.MetadataEndpoint + "events/maintenance/scheduled")
	if err != nil {
		log.Errorf("Failed to fetch data from metadata service: %s", err)
		return []prometheus.Metric{prometheus.MustNewConstMetric(c.ScheduledScrapeSuccessful, prometheus.GaugeValue, 0, instanceId)}
	}
	if resp.StatusCode != 200 {
		log.Debug("instance-action endpoint not found")
		return []prometheus.Metric{prometheus.MustNewConstMetric(c.ScheduledScrapeSuccessful, prometheus.GaugeValue, 0, instanceId)}
	}
	defer resp.Body.Close()

	ret = append(ret, prometheus.MustNewConstMetric(c.ScheduledScrapeSuccessful, prometheus.GaugeValue, 1, instanceId))

	body, _ := ioutil.ReadAll(resp.Body)

	var events []ScheduledEvent
	err = json.Unmarshal(body, &events)
	if err != nil {
		log.Errorf("Couldn't parse instance-action metadata: %s", err)
		ret = append(ret, prometheus.MustNewConstMetric(c.ScheduledActionIndicator, prometheus.GaugeValue, 0, instanceId))
		return ret
	}

	for _, event := range events {
		log.Infof("Scheduled instance even between %v and %v - %s", event.NotBefore, event.NotAfter, event.Description)
		ret = append(ret, prometheus.MustNewConstMetric(c.ScheduledActionIndicator, prometheus.GaugeValue, 1, event.Code, instanceId))

		delta_start := event.NotBefore.Sub(time.Now())
		delta_end := event.NotAfter.Sub(time.Now())

		if delta_start.Seconds() > 0 {
			ret = append(ret, prometheus.MustNewConstMetric(c.ScheduledActionStartTime, prometheus.GaugeValue, delta_start.Seconds(), event.Code, instanceId))
		}

		if delta_end.Seconds() > 0 {
			ret = append(ret, prometheus.MustNewConstMetric(c.ScheduledActionEndTime, prometheus.GaugeValue, delta_end.Seconds(), event.Code, instanceId))
		}
	}

	return ret
}

// spot_termination
//
// Retrieves spot termination info
func (c *Collector) spot_termination(client *http.Client, instanceId string) []prometheus.Metric {
	var ret []prometheus.Metric

	// Read Spot instance termination metadata
	resp, err := client.Get(c.MetadataEndpoint + "spot/instance-action")
	if err != nil {
		log.Errorf("Failed to fetch data from metadata service: %s", err)
		return append(ret, prometheus.MustNewConstMetric(c.SpotScrapeSuccessful, prometheus.GaugeValue, 0, instanceId))
	} else if resp.StatusCode == 404 {
		log.Debug("instance-action endpoint not found")
		return append(ret, prometheus.MustNewConstMetric(c.SpotTerminationIndicator, prometheus.GaugeValue, 0, "", instanceId))
	}
	defer resp.Body.Close()

	ret = append(ret, prometheus.MustNewConstMetric(c.SpotScrapeSuccessful, prometheus.GaugeValue, 1, instanceId))
	body, _ := ioutil.ReadAll(resp.Body)

	var ia = InstanceAction{}
	err = json.Unmarshal(body, &ia)
	if err != nil {
		log.Errorf("Couldn't parse instance-action metadata: %s", err)
		return append(ret, prometheus.MustNewConstMetric(c.SpotTerminationIndicator, prometheus.GaugeValue, 0, "", instanceId))
	}

	log.Infof("instance-action endpoint available, termination time: %v", ia.Time)
	ret = append(ret, prometheus.MustNewConstMetric(c.SpotTerminationIndicator, prometheus.GaugeValue, 1, ia.Action, instanceId))

	delta := ia.Time.Sub(time.Now())
	if delta.Seconds() > 0 {
		ret = append(ret, prometheus.MustNewConstMetric(c.SpotTerminationTime, prometheus.GaugeValue, delta.Seconds(), instanceId))
	}

	return ret
}
