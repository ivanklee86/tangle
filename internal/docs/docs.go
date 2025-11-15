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
	// Labels to filter applications by
	// in: query
	// required: false
	Labels string `json:"labels"`

	// Labels to exclude
	// in: query
	// required: false
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
//   500: errorResponse

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
