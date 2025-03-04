### API

Swagger documentation can be found at `/swagger`.

### Web UI

The Web UI is accessible at `/`.

**/applications** takes the following URL parameters.  These can be hard-coded in your CI/CD system (e.g. based on environment variables).

| Label | Values | example | Description |
|-------|--------|---------|-------------|
| labels | key1:value1,key2:value2 | /applications?labels=foo:bar | Labels on `Applications` to use for searching |
| refresh | true/false | /applications?refresh:true | Toggle periodic updates on/off. |

**/diffs** takes the following URL parameters.  These can be hard-coded in your CI/CD system (e.g. based on environment variables).

| Label | Values | example | Description |
|-------|--------|---------|-------------|
| labels | key1:value1,key2:value2 | /applications?labels=foo:bar | Labels on `Applications` to use for searching |
| targetRef | git ref | /diffs?targetRef=your_branch | Git reference to diff against |
