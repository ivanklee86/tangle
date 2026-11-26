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
| targetRef | branch, tag or commit SHA | /diffs?targetRef=your_branch | Git reference to diff against |
| targetRepo | repo URL, repeatable | `/diffs?targetRef=your_branch&targetRepo=https://github.com/org/values` | Repo the target ref belongs to. Sources in it get the ref, every other source stays at its live revision, and applications with no source in it are *not affected* (hidden unless "Show not affected" is on) |
| inferTargetRepo | `true` | /diffs?targetRef=your_branch&inferTargetRepo=true | With no `targetRepo`, let a multi-source application whose git sources are all in one repo use that repo |

`labels` and `excludeLabels` follow the same rules and the same `400` responses as on `/applications`.

An application with several sources (`spec.sources`) needs to know which repo the target ref was pushed to. Without `targetRepo` (or `inferTargetRepo`), the Diffs page marks it *Needs repo*, and its detail pane lists the candidate repos with a button that adds them to the link and re-runs every diff. The detail pane of a multi-source diff shows each source with the revision each side used, and whether it moved. On the Applications page, a multi-source row's Live ref column says how many sources it has and expands to list them, including which sources read files through a `ref` source. The target repos field suggests the repos the listed applications use, and warns about one none of them use. See [ADR 0032](adrs/0032-choose-multi-source-revisions-by-target-repo.md).
