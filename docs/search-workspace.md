# Elasticsearch / OpenSearch workspace

## Status

Implementation is deployed to staging as `liaison/liaison:search-recovery2-20260912`;
manager health checks passed. Elasticsearch 8.17.0 and OpenSearch 2.19.0 both
passed real engine integration and 12 connector/API/Agent/browser checks each.

### Staging capacity notes

September 12 follow-up: the Elasticsearch demo is restored for ongoing
review with a persistent volume and automatic restart. Oracle/ClickHouse demos
are paused with user approval to reserve memory; OpenSearch's demo entry is
explicitly stopped rather than left looking available. Anonymous search profiles
now display "Anonymous access". Failed connections render a persistent error and
Reconnect / Back to connections actions instead of an indefinite spinner.
The failure/retry/navigation UI regression lives in
`web/e2e/data-connection-recovery.cjs`.
The regression passed simulated failure, no automatic retry loop, manual retry,
back navigation, anonymous labeling and reconnecting to the real instance.
All 12 Elasticsearch E2E checks passed again on the deployed recovery build.
The retained `liaison-demo-events` index contains three explicitly labeled demo
documents for manual review. Do not remove this review instance after tests.

Old dependency/build caches were removed (about 4.4 GiB), preserving databases,
volumes and rollback images. Elasticsearch 8.17.0 was pulled and started with a
768 MiB memory limit and a loopback-only port. The first connector connection
failed while it was initializing. Subsequently SSH and HTTPS stopped responding;
resource pressure is suspected but not confirmed. Initial remote stop attempts
timed out; a subsequent kill succeeded. The test container is confirmed exited
(not marked OOM-killed), SSH and HTTPS recovered, and manager returned the
expected unauthenticated 401 response. OpenSearch was not started. Do not restart
the search test container on this host without additional memory headroom.
With user approval, the Oracle and ClickHouse demo containers were subsequently
stopped, increasing available memory to about 2.5 GiB. Elasticsearch 8.17.0 passed
real integration and 12 connector/API/Agent/browser checks. Its test-only disk
watermarks were set to 500/300/100 MiB because default percentage thresholds
prevented primary shard allocation. The temporary Elasticsearch container and
downloadable image were removed to make room for the OpenSearch test image.
OpenSearch used a 1 GiB container limit and 256 MiB JVM heap. It required the same
test-only watermarks and clearing the create-index block left by its initial
disk check. Temporary indices and test users were removed. OpenSearch is stopped
after acceptance; Oracle and ClickHouse are restored. Existing database volumes
were preserved. No production deployment.

## Implemented

- Separate Elasticsearch and OpenSearch application / browser access entries;
  default backend port 9200. Uses REST HTTP(S), not the transport port 9300.
- Connector-scoped HTTP transport with Basic authentication or anonymous access;
  existing per-user encrypted saved credentials. No node discovery, proxy from
  environment, alternate hosts, or redirects. TLS verifies certificates unless
  the user explicitly selects skip-verify.
- Index navigation, Mapping JSON and field list, bounded search preview.
- JSON request editor, search hits table, expandable complete JSON response
  including aggregations. Index/document writes use explicit requests, not
  SQL or MongoDB commands.
- Existing attached-session Agent tools, approval, access checks and audit;
  separate AI draft assistance. Query tools still require approval for reads.

Example (substitute an index visible to the connected account):

```json
{
  "method": "POST",
  "path": "/logs/_search",
  "body": { "size": 20, "query": { "match_all": {} } }
}
```

## Deliberate boundaries

Accepted routes: `/index` GET/PUT/DELETE, `/index/_mapping` GET/PUT,
`/index/_search` and `/index/_count` GET/POST, `/index/_doc` POST,
`/index/_doc/id` GET/PUT/DELETE, `/index/_update/id` POST.

Only concrete ordinary index names and simple document IDs are currently
accepted. System/dot-prefixed indices, wildcards, query parameters, bulk/NDJSON,
cluster/security APIs, snapshots and remote reindex are not exposed. No API key,
SigV4 or custom CA upload UI in this iteration. A default index narrows navigation,
not authorization; the backend account controls access to other explicit indices.

Search size defaults to 100 and is limited to 500. Responses are capped at 4 MiB;
metadata lists at most 200 indices and 500 fields. Timeout and failed shards are
reported as incomplete, not success. Writes are not automatically retried and
may not be immediately searchable until the engine refreshes.

## Verification

Passed: backend unit/race regression for web, Agent and controlplane; frontend
protocol checks and build. Includes request path rejection, JSON shape, response
limits, errors, cancellation, credentials and session close. TLS verification and
redirect rejection have dedicated local HTTP-server tests.

Elasticsearch passed real engine integration with race detection, connector-path
CRUD/Mapping/search/aggregation, blocked endpoint recovery, real-model schema and
query approval/denial, two-user session isolation, browser query and mobile width,
document deletion and closed-session rejection. Temporary indices and test users
were cleaned up. Screenshot inspection found result-table compression and a JSON
success-toast issue; both were fixed and deployed with a corrected index counter.
OpenSearch passed the same real integration and 12 E2E checks. Dark-mode screenshot
inspection verified the UI fixes, and the E2E now asserts that expanding JSON
does not compress the result row. The complete backend race regression and
frontend build/protocol checks also passed. This is search-specific acceptance,
not a rerun of every existing protocol's E2E suite.

Artifacts (local, not committed): `/tmp/liaison-elasticsearch-acceptance.json`,
`/tmp/liaison-opensearch-acceptance.json`, and
`/tmp/liaison-opensearch-workspace.png`. The Elasticsearch screenshot predates the
shared UI fixes; the OpenSearch screenshot shows the corrected layout.

Opt-in real-instance tests create a uniquely named temporary index and remove it:

```sh
TEST_ELASTICSEARCH_ADDRESS=127.0.0.1:19200 \
TEST_OPENSEARCH_ADDRESS=127.0.0.1:19201 \
go test -race ./pkg/liaison/manager/web -run TestSearchIntegration -count=1 -v
```

Optional `TEST_ELASTICSEARCH_USERNAME`, `_PASSWORD`, `_TLS_MODE` and equivalent
OpenSearch variables configure authentication/TLS. Use isolated test instances,
never production. Missing addresses explicitly skip tests.

Protocol references: [Elasticsearch Mapping API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-indices-get-mapping),
[OpenSearch Search API](https://docs.opensearch.org/latest/api-reference/search-apis/search/).
