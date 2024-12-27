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

	// Name of the ArgoCD instance
	// in: query
	// required: false
	Instance string `json:"instance"`
}

// swagger:route GET /applications labels
// Get information about all Applications with matching tags.
//
// Responses:
//   200: applicationsResponse

// Response for successful application lookup
// swagger:response applicationsResponse
type applicationsResponse struct {
	// in: body
	Body tangle.ApplicationsResponse
}
