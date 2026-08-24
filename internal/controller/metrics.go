// SPDX-License-Identifier: AGPL-3.0-only

// Package controller defines and registers the Prometheus metrics for the
// locations controllers. Metric names keep the "nso_" prefix they carried in
// network-services-operator so existing dashboards and alerts survive the
// move. They are registered against prometheus.DefaultRegisterer, the same
// registry controller-runtime exposes at /metrics.
package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricLabelLocation = "location"
	metricLabelReason   = "reason"
)

var (
	// locationSourceTotal and locationPublishedTotal are sampled in one pass.
	// The difference between them is the count of locations not published:
	//   nso_location_source_total - nso_location_published_total
	locationSourceTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nso_location_source_total",
			Help: "Locations at the source that carry a city code and are therefore publishable.",
		},
	)

	locationPublishedTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nso_location_published_total",
			Help: "ServingLocations this publisher currently owns on the federation hub, excluding copies retained by a blocked removal.",
		},
	)

	// locationRetainedTotal counts copies kept because their removal is
	// blocked. They are excluded from locationPublishedTotal.
	locationRetainedTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nso_location_retained_total",
			Help: "Published ServingLocations retained because their removal is blocked. Alert on a threshold measured in days, not minutes.",
		},
	)

	locationPublishTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nso_location_publish_timestamp_seconds",
			Help: "Unix timestamp at which this location's published content last changed.",
		},
		[]string{metricLabelLocation},
	)

	locationRemovalBlocked = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nso_location_removal_blocked",
			Help: "1 while a published ServingLocation is retained because its removal guard refuses the delete, by reason.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)

	locationMatchedClusters = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nso_location_matched_clusters",
			Help: "Ready hub clusters labelled for this location. Zero means the generated policy places the copy nowhere.",
		},
		[]string{metricLabelLocation},
	)

	locationPublishMismatch = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nso_location_publish_mismatch",
			Help: "1 while a source Location cannot be published, by reason. A refusal is reported, never silently skipped.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)

	locationPublisherConflictsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nso_location_publisher_conflicts_total",
			Help: "Conflicts observed by the location publisher, by location and reason.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)
)
