### API

Swagger documentation can be found at `/swagger`.

### Web UI

The Web UI is accessible at `/`.  You can specify labels to include and exclude by filling in the key/value boxes and hitting enter or `+`.

**/applications** takes the following URL parameters.  These can be hard-coded in your CI/CD system (e.g. based on environment variables).

| Label | Values | example | Description |
| ------- | -------- | --------- | ------------- |
| labels | key1:value1,key2:value2 | /applications?labels=foo:bar | Labels on `Applications` to use for searching |
| excludeLabels | key1:value1,key2:value2 | /applications?excludeLabels=foo:bar | Labels on `Applications` to exclude |
| refresh | true/false | /applications?refresh:true | Toggle periodic updates on/off. |

`labels` and `excludeLabels` are comma-separated `key:value` pairs. An application must carry every
`labels` pair to be returned, and is omitted if it carries any `excludeLabels` pair. The API rejects a
request with `400 Bad Request` — naming the offending segment and parameter — when a segment isn't
exactly `key:value` with both parts non-empty, when a key appears more than once within either
parameter, or when a key appears in both parameters with the same value (which no application could
match). A key in both with *different* values is allowed.

**/diffs** takes the following URL parameters.  These can be hard-coded in your CI/CD system (e.g. based on environment variables).

| Label | Values | example | Description |
| ------- | -------- | --------- | ------------- |
| labels | key1:value1,key2:value2 | /applications?labels=foo:bar | Labels on `Applications` to use for searching |
| excludeLabels | key1:value1,key2:value2 | /applications?excludeLabels=foo:bar | Labels on `Applications` to exclude |
| targetRef | git ref | /diffs?targetRef=your_branch | Git reference to diff against |

`labels` and `excludeLabels` follow the same rules and the same `400` responses as on `/applications`.
