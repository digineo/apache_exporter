package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// For Prometheus metrics.
const namespace = "apache"

// Exporter holds metrics for a single target.
type Exporter struct {
	Context context.Context
	URI     string

	up             *prometheus.Desc
	scrapeFailures prometheus.Counter
	accessesTotal  *prometheus.Desc
	bytesTotal     *prometheus.Desc
	cpuload        prometheus.Gauge
	uptime         *prometheus.Desc
	workers        *prometheus.GaugeVec
	scoreboard     *prometheus.GaugeVec
	connections    *prometheus.GaugeVec

	sync.Mutex // To protect metrics from concurrent collects.
}

// NewExporter returns a new exporter for the given target uri.
func NewExporter(ctx context.Context, uri string) *Exporter {
	return &Exporter{
		Context: ctx,
		URI:     uri,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the apache server be reached",
			nil,
			nil),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping apache.",
		}),
		accessesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "accesses_total"),
			"Current total apache accesses (*)",
			nil,
			nil),
		bytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "sent_bytes_total"),
			"Current total bytes sent (*)",
			nil,
			nil),
		cpuload: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cpuload",
			Help:      "The current percentage CPU used by each worker and in total by all workers combined (*)",
		}),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uptime_seconds_total"),
			"Current uptime in seconds (*)",
			nil,
			nil),
		workers: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "workers",
			Help:      "Apache worker statuses",
		}, []string{"state"}),
		scoreboard: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "scoreboard",
			Help:      "Apache scoreboard statuses",
		}, []string{"state"}),
		connections: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "connections",
			Help:      "Apache connection statuses",
		}, []string{"state"}),
	}
}

// Describe implements the prometheus.Collector interface
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.accessesTotal
	ch <- e.bytesTotal
	ch <- e.uptime
	e.cpuload.Describe(ch)
	e.scrapeFailures.Describe(ch)
	e.workers.Describe(ch)
	e.scoreboard.Describe(ch)
	e.connections.Describe(ch)
}

// splitkv splits colon separated string into two fields
func splitkv(s string) (string, string) {
	if len(s) == 0 {
		return s, s
	}

	slice := strings.SplitN(s, ":", 2)
	if len(slice) == 1 {
		return slice[0], ""
	}
	return strings.TrimSpace(slice[0]), strings.TrimSpace(slice[1])
}

var scoreboardLabelMap = map[rune]string{
	'_': "idle",
	'S': "startup",
	'R': "read",
	'W': "reply",
	'K': "keepalive",
	'D': "dns",
	'C': "closing",
	'L': "logging",
	'G': "graceful_stop",
	'I': "idle_cleanup",
	'.': "open_slot",
}

func (e *Exporter) updateScoreboard(scoreboard string) {
	e.scoreboard.Reset()
	for _, v := range scoreboardLabelMap {
		e.scoreboard.WithLabelValues(v)
	}

	for _, status := range scoreboard {
		label, ok := scoreboardLabelMap[status]
		if !ok {
			label = string(status)
		}
		e.scoreboard.WithLabelValues(label).Inc()
	}
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	req, err := http.NewRequest("GET", e.URI, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(e.Context)

	resp, err := client.Do(req)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		return fmt.Errorf("Error scraping apache: %v", err)
	}
	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)

	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		if err != nil {
			data = []byte(err.Error())
		}
		return fmt.Errorf("Status %s (%d): %s", resp.Status, resp.StatusCode, data)
	}

	lines := strings.Split(string(data), "\n")

	connectionInfo := false

	for _, l := range lines {
		key, v := splitkv(l)
		if err != nil {
			continue
		}

		var val float64
		var err error

		switch key {
		case "Total Accesses":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(e.accessesTotal, prometheus.CounterValue, val)
			}
		case "Total kBytes":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(e.bytesTotal, prometheus.CounterValue, val*1024)
			}
		case "CPULoad":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.cpuload.Set(val)
			}
		case "Uptime":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(e.uptime, prometheus.CounterValue, val)
			}
		case "BusyWorkers":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.workers.WithLabelValues("busy").Set(val)
			}
		case "IdleWorkers":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.workers.WithLabelValues("idle").Set(val)
			}
		case "Scoreboard":
			e.updateScoreboard(v)
			e.scoreboard.Collect(ch)
		case "ConnsTotal":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.connections.WithLabelValues("total").Set(val)
				connectionInfo = true
			}
		case "ConnsAsyncWriting":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.connections.WithLabelValues("writing").Set(val)
				connectionInfo = true
			}
		case "ConnsAsyncKeepAlive":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.connections.WithLabelValues("keepalive").Set(val)
				connectionInfo = true
			}
		case "ConnsAsyncClosing":
			val, err = strconv.ParseFloat(v, 64)
			if err == nil {
				e.connections.WithLabelValues("closing").Set(val)
				connectionInfo = true
			}
		}

		if err != nil {
			return err
		}
	}

	e.cpuload.Collect(ch)
	e.workers.Collect(ch)
	if connectionInfo {
		e.connections.Collect(ch)
	}

	return nil
}

// Collect implements the prometheus.Collector interface
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.Lock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping target '%s': %s", e.URI, err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
	e.Unlock()
	return
}
