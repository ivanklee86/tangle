// Documentation of the Tangle API.
//
//	Schemes: http
//	BasePath: /
//	Version: 1.0.0
//	Host: localhost:8080
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
// swagger:meta
package docs

import "github.com/ivanklee86/tangle/internal/tangle"

// swagger:route GET /applications test-tag idOfBooksEndpoint
// Get information about all Applications with matching tags.
//
// responses:
//   200: foobarResponse

// This text will appear as description of your response body.
// swagger:response foobarResponse
type applicationsResponseWrapper struct {
	// in:body
	Body tangle.ApplicationsResponse
}
