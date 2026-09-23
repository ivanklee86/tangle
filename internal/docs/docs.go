// Documentation of the Tangle API.
//
//	Schemes: http
//	BasePath: /
//	Version: 1.0.0
//	Host: localhost:8081
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
// swagger:meta

// nolint
package docs

import "github.com/ivanklee86/tangle/internal/tangle"

// swagger:parameters labels
type applicationsQueryParams struct {
	// Labels to filter applications by, as comma-separated key:value pairs
	// (e.g. `env:test,team:platform`). Applications must carry all of them.
	// Keys and values must be valid Kubernetes label keys and values, each key
	// may appear at most once, and a key may not appear here and in
	// excludeLabels with the same value. Anything else is a 400.
	// in: query
	// required: false
	// example: env:test,team:platform
	Labels string `json:"labels"`

	// Labels to exclude, as comma-separated key:value pairs (e.g.
	// `env:prod`). Applications carrying any of them are omitted. Same rules
	// as labels: valid Kubernetes label keys and values, each key at most
	// once, and no key shared with labels at the same value. Anything else is
	// a 400.
	// in: query
	// required: false
	// example: env:prod
	ExcludeLabels string `json:"excludeLabels"`

	// Name of the ArgoCD instance
	// in: query
	// required: false
	Instance string `json:"instance"`
}

// swagger:route GET /api/applications labels
// Get information about all Applications with matching tags.
//
// Responses:
//   200: applicationsResponse
//   400: errorResponse
//   500: errorResponse

// swagger:route GET /api/config config
// Get the settings the web UI can only learn at runtime.
//
// Responses:
//   200: configResponse

// Runtime configuration for the web UI
// swagger:response configResponse
type configResponse struct {
	// in: body
	Body tangle.ConfigResponse
}

// Response for successful application lookup
// swagger:response applicationsResponse
type applicationsResponse struct {
	// in: body
	Body tangle.ApplicationsResponse
}

// swagger:route POST /api/argocd/{ArgoCD}/applications/{Name}/diffs diffsRequestParams
// Get manifests and diffs for an application.
// consumes:
// - application/json
// produces:
// - application/json
// Responses:
//   200: diffsResponse
//   500: errorResponse

// swagger:parameters diffsRequestParams
type diffsRequestParams struct {
	// ArgoCD instance name
	// in: path
	// required: true
	ArgoCD string

	// Application name
	// in: path
	// required: true
	Name string

	// in: body
	// required: true
	// swagger:model diffReqeust
	Body tangle.DiffsRequest
}

// Response for successful diffs generation
// swagger:response diffsResponse
type diffsResponse struct {
	// in: body
	Body tangle.DiffsResponse
}

// Response for error
// swagger:response errorResponse
type errorResponse struct {
	// in: body
	Body tangle.ErrorResponse
}
