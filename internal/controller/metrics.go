// SPDX-License-Identifier: AGPL-3.0-only

// Package controller defines and registers the Prometheus metrics for the
// locations controllers. They are registered against
// prometheus.DefaultRegisterer, the same registry controller-runtime exposes
// at /metrics.
package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	metricLabelLocation = "location"
	metricLabelClass    = "class"
	metricLabelReason   = "reason"
)

var (
	// locationsPublishable and locationsPublished are sampled in one pass.
	// The difference between them is the count of locations not published:
	//   locations_publishable - locations_published
	locationsPublishable = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "locations_publishable",
			Help: "Locations at the source that carry a city code and are therefore publishable.",
		},
	)

	locationsPublished = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "locations_published",
			Help: "ServingLocations this publisher currently owns on the federation hub, excluding copies retained by a blocked removal.",
		},
	)

	// locationsRetained counts copies kept because their removal is blocked.
	// They are excluded from locationsPublished.
	locationsRetained = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "locations_retained",
			Help: "Published ServingLocations retained because their removal is blocked. Alert on a threshold measured in days, not minutes.",
		},
	)

	locationPublishTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_publish_timestamp_seconds",
			Help: "Unix timestamp at which this location's published content last changed.",
		},
		[]string{metricLabelLocation},
	)

	locationRemovalBlocked = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_removal_blocked",
			Help: "1 while a published ServingLocation is retained because its removal guard refuses the delete, by reason.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)

	locationMatchedClusters = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_matched_clusters",
			Help: "Ready hub clusters labelled for this location. Zero means the generated policy places the copy nowhere.",
		},
		[]string{metricLabelLocation},
	)

	locationPublishMismatch = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_publish_mismatch",
			Help: "1 while a source Location cannot be published, by reason. A refusal is reported, never silently skipped.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)

	locationClassDeletionBlocked = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locations_class_deletion_blocked",
			Help: "1 while a deleted LocationClass is held by its finalizer because Locations still name it, by reason.",
		},
		[]string{metricLabelClass, metricLabelReason},
	)

	locationPublisherConflictsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locations_publisher_conflicts_total",
			Help: "Conflicts observed by the location publisher, by location and reason.",
		},
		[]string{metricLabelLocation, metricLabelReason},
	)
)
