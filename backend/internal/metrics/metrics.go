package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Wiki metrics
	WikisTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wikis_total",
			Help: "Total number of wikis",
		},
	)

	WikisActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wikis_active",
			Help: "Number of active wikis (is_active=true)",
		},
	)

	WikisStatusOK = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wikis_status_ok",
			Help: "Number of wikis with status=ok",
		},
	)

	WikisStatusError = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wikis_status_error",
			Help: "Number of wikis with status=error",
		},
	)

	WikisWithArchive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wikis_with_archive",
			Help: "Number of wikis with archives",
		},
	)

	// Collection scheduler metrics
	CollectionCycleTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "collection_cycle_total",
			Help: "Total number of collection cycles completed",
		},
	)

	CollectionCycleDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "collection_cycle_duration_seconds",
			Help:    "Duration of collection cycle in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	CollectionWikisProcessed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "collection_wikis_processed_total",
			Help: "Total number of wikis processed during collection",
		},
	)

	CollectionWikisFailed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "collection_wikis_failed_total",
			Help: "Total number of wikis that failed during collection",
		},
	)

	CollectionNextRun = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "collection_next_run_timestamp",
			Help: "Unix timestamp of next collection run",
		},
	)

	// Archive scheduler metrics
	ArchiveCheckCycleTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "archive_check_cycle_total",
			Help: "Total number of archive check cycles completed",
		},
	)

	ArchiveCheckDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "archive_check_duration_seconds",
			Help:    "Duration of archive check cycle in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	ArchiveWikisChecked = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "archive_wikis_checked_total",
			Help: "Total number of wikis checked for archives",
		},
	)

	ArchivesFound = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "archives_found_total",
			Help: "Total number of archives found",
		},
	)

	ArchiveNextRun = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "archive_check_next_run_timestamp",
			Help: "Unix timestamp of next archive check run",
		},
	)
)
