local g = import 'github.com/grafana/grafonnet/gen/grafonnet-v11.4.0/main.libsonnet';

local prometheus = g.query.prometheus;
local stat = g.panel.stat;
local table = g.panel.table;
local timeSeries = g.panel.timeSeries;
local row = g.panel.row;
local var = g.dashboard.variable;

local ds = '${datasource}';

local job = 'job=~"$job"';
local ctl = job + ',controller=~"$controller"';

local target(expr, legend='') =
  prometheus.new(ds, expr)
  + prometheus.withLegendFormat(legend);

local instant(expr, refId) =
  prometheus.new(ds, expr)
  + prometheus.withInstant(true)
  + prometheus.withFormat('table')
  + prometheus.withRefId(refId);

local pos(h, w, x, y) = { gridPos: { h: h, w: w, x: x, y: y } };

local steps(list) = [{ color: s[0], value: s[1] } for s in list];

local statPanel(title, description, expr, unit, thresholds, x, y) =
  stat.new(title)
  + pos(4, 4, x, y)
  + stat.panelOptions.withDescription(description)
  + stat.queryOptions.withDatasource('prometheus', ds)
  + stat.queryOptions.withTargets([target(expr)])
  + stat.options.withColorMode('background')
  + stat.options.withGraphMode('area')
  + stat.options.reduceOptions.withCalcs(['lastNotNull'])
  + stat.standardOptions.withUnit(unit)
  + stat.standardOptions.color.withMode('thresholds')
  + stat.standardOptions.thresholds.withMode('absolute')
  + stat.standardOptions.thresholds.withSteps(steps(thresholds));

local ts(title, description, targets, unit, h, w, x, y, stacked=false) =
  timeSeries.new(title)
  + pos(h, w, x, y)
  + timeSeries.panelOptions.withDescription(description)
  + timeSeries.queryOptions.withDatasource('prometheus', ds)
  + timeSeries.queryOptions.withTargets(targets)
  + timeSeries.standardOptions.withUnit(unit)
  + timeSeries.fieldConfig.defaults.custom.withFillOpacity(if stacked then 30 else 10)
  + timeSeries.fieldConfig.defaults.custom.withShowPoints('never')
  + timeSeries.fieldConfig.defaults.custom.stacking.withMode(if stacked then 'normal' else 'none')
  + timeSeries.options.tooltip.withMode('multi')
  + timeSeries.options.tooltip.withSort('desc')
  + timeSeries.options.legend.withDisplayMode('table')
  + timeSeries.options.legend.withPlacement('right')
  + timeSeries.options.legend.withCalcs(['mean', 'max'])
  + timeSeries.options.legend.withSortBy('Max')
  + timeSeries.options.legend.withSortDesc(true);

local tablePanel(title, description, targets, transformations, h, w, x, y) =
  table.new(title)
  + pos(h, w, x, y)
  + table.panelOptions.withDescription(description)
  + table.queryOptions.withDatasource('prometheus', ds)
  + table.queryOptions.withTargets(targets)
  + table.queryOptions.withTransformations(transformations);

local columnUnit(name, unit) = {
  matcher: { id: 'byName', options: name },
  properties: [{ id: 'unit', value: unit }],
};

local section(title, y) =
  row.new(title)
  + pos(1, 24, 0, y);

local glanceRow = section('At a glance', 0);

local glance = [
  stat.new('Running version')
  + pos(4, 4, 0, 1)
  + stat.panelOptions.withDescription('build_version reported by each running operator pod. Two values side by side mean a rollout is in progress or stuck.')
  + stat.queryOptions.withDatasource('prometheus', ds)
  + stat.queryOptions.withTargets([target('count by (build_version) (instrumentation_build_info{%s})' % job, '{{build_version}}')])
  + stat.options.withTextMode('name')
  + stat.options.withColorMode('none')
  + stat.options.withGraphMode('none')
  + stat.options.reduceOptions.withCalcs(['lastNotNull']),
  statPanel(
    'Not published',
    'Publishable locations at the source with no published copy on the federation hub.',
    'max(locations_publishable{%s}) - max(locations_published{%s})' % [job, job],
    'short',
    [['green', null], ['yellow', 1]],
    4,
    1,
  ),
  statPanel(
    'Retained',
    'Published copies kept on the hub because their removal is blocked.',
    'max(locations_retained{%s})' % job,
    'short',
    [['green', null], ['yellow', 1]],
    8,
    1,
  ),
  statPanel(
    'Removals blocked',
    'Locations whose hub copy the removal guard refuses to delete.',
    'count(max by (location) (locations_removal_blocked{%s}) == 1) or (0 * max(locations_retained{%s}))' % [job, job],
    'short',
    [['green', null], ['yellow', 1]],
    12,
    1,
  ),
  statPanel(
    'Publish mismatches',
    'Source locations the publisher refuses to publish.',
    'count(max by (location) (locations_publish_mismatch{%s}) == 1) or (0 * max(locations_publishable{%s}))' % [job, job],
    'short',
    [['green', null], ['red', 1]],
    16,
    1,
  ),
  statPanel(
    'Locations not ready',
    'Claimed locations reporting Ready other than True.',
    'count(max by (location) (locations_ready{%s}) == 0) or (0 * count(locations_ready{%s}))' % [job, job],
    'short',
    [['green', null], ['red', 1]],
    20,
    1,
  ),
];

local publishingRow = section('Publishing', 5);

local publishing = [
  ts(
    'Publishable, published and retained',
    'Locations at the source that can be published, copies the publisher owns on the hub, and copies retained by a blocked removal. Published below publishable is a location that is not reaching its cells.',
    [
      target('max(locations_publishable{%s})' % job, 'publishable'),
      target('max(locations_published{%s})' % job, 'published'),
      target('max(locations_retained{%s})' % job, 'retained'),
    ],
    'short',
    8,
    12,
    0,
    6,
  ),
  ts(
    'Publisher conflicts',
    'Conflicts the publisher observed, by reason. UnlabelledHubObject is a hub copy it does not own; EmptySourceList is the source returning no locations while copies are held, which stops all pruning.',
    [target('sum by (reason) (increase(locations_publisher_conflicts_total{%s}[$__rate_interval]))' % job, '{{reason}}')],
    'short',
    8,
    12,
    12,
    6,
  ),
  tablePanel(
    'Blocked removals',
    'Locations deleted at the source whose hub copy is retained, and why the removal guard refuses the delete.',
    [instant('max by (location, reason) (locations_removal_blocked{%s}) == 1' % job, 'A')],
    [{ id: 'organize', options: { excludeByName: { Time: true, Value: true } } }],
    8,
    12,
    0,
    14,
  ),
  tablePanel(
    'Publish mismatches',
    'Source locations that cannot be published, by reason.',
    [instant('max by (location, reason) (locations_publish_mismatch{%s}) == 1' % job, 'A')],
    [{ id: 'organize', options: { excludeByName: { Time: true, Value: true } } }],
    8,
    12,
    12,
    14,
  ),
];

local readinessRow = section('Readiness', 22);

local readiness = [
  tablePanel(
    'Locations',
    'Each published or claimed location: whether it reports Ready, how many Ready hub clusters are labelled for it (zero places the copy nowhere), and how long since its published content last changed.',
    [
      instant('max by (location) (locations_ready{%s})' % job, 'A'),
      instant('max by (location) (locations_matched_clusters{%s})' % job, 'B'),
      instant('time() - max by (location) (locations_publish_timestamp_seconds{%s})' % job, 'C'),
    ],
    [
      { id: 'merge', options: {} },
      {
        id: 'organize',
        options: {
          excludeByName: { Time: true },
          renameByName: { 'Value #A': 'Ready', 'Value #B': 'Matched clusters', 'Value #C': 'Since last publish' },
        },
      },
    ],
    8,
    14,
    0,
    23,
  )
  + table.standardOptions.withOverrides([columnUnit('Since last publish', 's')])
  + table.options.withSortBy([{ displayName: 'Ready', desc: false }]),
  tablePanel(
    'Location classes',
    'LocationClasses naming this controller: whether each is accepted, and whether a deleted class is held by its finalizer because locations still name it.',
    [
      instant('max by (class) (locations_class_accepted{%s})' % job, 'A'),
      instant('max by (class) (locations_class_deletion_blocked{%s})' % job, 'B'),
    ],
    [
      { id: 'merge', options: {} },
      {
        id: 'organize',
        options: {
          excludeByName: { Time: true },
          renameByName: { 'Value #A': 'Accepted', 'Value #B': 'Deletion blocked' },
        },
      },
    ],
    8,
    10,
    14,
    23,
  ),
];

local controllersRow = section('Controllers', 31);

local controllers = [
  ts(
    'Reconciles by result',
    'Reconciles per second, split by outcome.',
    [target('sum by (result) (rate(controller_runtime_reconcile_total{%s}[$__rate_interval]))' % ctl, '{{result}}')],
    'ops',
    8,
    12,
    0,
    32,
    true,
  ),
  ts(
    'Reconcile errors by controller',
    'Failed reconciles per second for each controller.',
    [target('sum by (controller) (rate(controller_runtime_reconcile_errors_total{%s}[$__rate_interval]))' % ctl, '{{controller}}')],
    'ops',
    8,
    12,
    12,
    32,
  ),
  ts(
    'Reconcile time p95 by controller',
    'p95 time spent in one reconcile.',
    [target('histogram_quantile(0.95, sum by (controller, le) (rate(controller_runtime_reconcile_time_seconds_bucket{%s}[$__rate_interval])))' % ctl, '{{controller}}')],
    's',
    8,
    12,
    0,
    40,
  ),
  ts(
    'Queue depth',
    'Items waiting to be reconciled.',
    [target('sum by (controller) (workqueue_depth{%s})' % ctl, '{{controller}}')],
    'short',
    8,
    12,
    12,
    40,
  ),
];

local datasourceVar =
  var.datasource.new('datasource', 'prometheus')
  + var.datasource.generalOptions.withLabel('Data Source');

local jobVar =
  var.query.new('job')
  + var.query.generalOptions.withLabel('Job')
  + var.query.withDatasource('prometheus', ds)
  + var.query.queryTypes.withLabelValues('job', 'controller_runtime_reconcile_total{controller=~"location_.*"}')
  + var.query.refresh.onTime()
  + var.query.withSort(1);

local controllerVar =
  var.query.new('controller')
  + var.query.generalOptions.withLabel('Controller')
  + var.query.withDatasource('prometheus', ds)
  + var.query.queryTypes.withLabelValues('controller', 'controller_runtime_reconcile_total{%s}' % job)
  + var.query.refresh.onTime()
  + var.query.withSort(1)
  + var.query.selectionOptions.withMulti(true)
  + var.query.selectionOptions.withIncludeAll(true)
  + var.query.generalOptions.withCurrent('All', '$__all');

g.dashboard.new('Locations')
+ g.dashboard.withUid('locations')
+ g.dashboard.withDescription('Whether the locations operator is publishing locations to the federation hub, reporting their readiness, and reconciling: publishing, readiness, controller health and the running version.')
+ g.dashboard.withTags(['locations', 'milo'])
+ g.dashboard.withRefresh('1m')
+ g.dashboard.time.withFrom('now-6h')
+ g.dashboard.withVariables([datasourceVar, jobVar, controllerVar])
+ g.dashboard.withPanels(
  [glanceRow] + glance
  + [publishingRow] + publishing
  + [readinessRow] + readiness
  + [controllersRow] + controllers,
  setPanelIDs=true,
)
