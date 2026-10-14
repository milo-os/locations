// SPDX-License-Identifier: AGPL-3.0-only

// Package controller defines and registers the Prometheus metrics for the
// locations controllers. They are created with
// go.miloapis.com/instrumentation/metrics, which registers them on the
// registry controller-runtime serves at /metrics.
package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.miloapis.com/instrumentation/labels"
	"go.miloapis.com/instrumentation/metrics"
)

const (
	metricLabelLocation = "location"
	metricLabelClass    = "class"
	metricLabelReason   = labels.Reason
)

var (
	// locationsPublishable and locationsPublished are sampled in one pass.
	// The difference between them is the count of locations not published:
	//   locations_publishable - locations_published
	locationsPublishable = metrics.NewGauge(
		prometheus.GaugeOpts{
			Name: "locations_publishable",
			Help: "Locations at the source that carry a city code and are therefore publishable.",
		},
	)

	locationsPublished = metrics.NewGauge(
		prometheus.GaugeOpts{
			Name: "locations_published",
			Help: "ServingLocations this publisher currently owns on the federation hub, excluding copies retained by a blocked removal.",
		},
	)

	// locationsRetained counts copies kept because their removal is blocked.
	// They are excluded from locationsPublished.
	locationsRetained = metrics.NewGauge(
		prometheus.GaugeOpts{
			Name: "locations_retained",
			Help: "Published ServingLocations retained because their removal is blocked. Alert on a threshold measured in days, not minutes.",
		},
	)

	locationPublishTimestamp = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_publish_timestamp_seconds",
			Help: "Unix timestamp at which this location's published content last changed.",
		},
		[]string{metricLabelLocation},
	)

	locationRemovalBlocked = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_removal_blocked",
			Help: "1 while a published ServingLocation is retained because its removal guard refuses the delete, by reason.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)

	locationMatchedClusters = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_matched_clusters",
			Help: "Ready hub clusters labelled for this location. Zero means the generated policy places the copy nowhere.",
		},
		[]string{metricLabelLocation},
	)

	locationPublishMismatch = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_publish_mismatch",
			Help: "1 while a source Location cannot be published, by reason. A refusal is reported, never silently skipped.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)

	locationReady = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_ready",
			Help: "1 while a claimed location reports Ready=True, 0 while it does not. Locations this controller does not claim are absent.",
		},
		[]string{metricLabelLocation},
	)

	locationClassAccepted = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_class_accepted",
			Help: "1 while a LocationClass naming this controller is accepted by it. Classes naming another controller are absent.",
		},
		[]string{metricLabelClass},
	)

	locationClassDeletionBlocked = metrics.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_class_deletion_blocked",
			Help: "1 while a deleted LocationClass is held by its finalizer because Locations still name it, by reason.",
		},
		[]string{metricLabelClass, metricLabelReason},
	)

	locationPublisherConflictsTotal = metrics.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locations_publisher_conflicts_total",
			Help: "Conflicts observed by the location publisher, by location and reason.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)
)
