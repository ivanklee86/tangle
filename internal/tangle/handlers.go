package tangle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type ApplicationLinks struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Health     string `json:"health"`
	SyncStatus string `json:"syncStatus"`
	// liveRef, not LiveRef: every sibling field here is lowerCamelCase, and
	// so is the web UI's own ApplicationLinks type and the e2e fixtures that
	// stand in for this response. The capitalised tag meant the field the
	// frontend read was always undefined — invisible until a column actually
	// displayed it.
	LiveRef string `json:"liveRef"`
}

type ArgoCDApplicationResults struct {
	Name         string             `json:"name"`
	Link         string             `json:"link"`
	Applications []ApplicationLinks `json:"applications"`
}

type ApplicationsResponse struct {
	Results []ArgoCDApplicationResults `json:"results"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// DiffsRequest contains the git refs to compare
// swagger:model DiffsRequest
type DiffsRequest struct {
	// Current git ref to compare from
	// required: true
	// example: main
	LiveRef string `json:"liveRef"`

	// Target git ref to compare to
	// required: true
	// example: feature-branch
	TargetRef string `json:"targetRef"`
}

type DiffsResponse struct {
	LiveManifests           string `json:"liveManifests"`
	TargetManifests         string `json:"targetManifests"`
	Diffs                   string `json:"diffs"`
	ManifestGenerationError string `json:"manifestGenerationError"`
}

func (t *Tangle) sortResults(apiResults []ArgoCDApplicationResults) []ArgoCDApplicationResults {
	sortOrder := t.Config.SortOrder
	if len(sortOrder) == 0 {
		return apiResults
	}

	sortedResults := []ArgoCDApplicationResults{}
	for _, name := range sortOrder {
		for _, result := range apiResults {
			if result.Name == name {
				sortedResults = append(sortedResults, result)
			}
		}
	}

	return sortedResults

}

// respondError writes err to the caller as the JSON ErrorResponse body the
// API documents, at the given status. Callers log first, with whatever
// context they have; this handles only the part the caller sees.
func (t *Tangle) respondError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()}) // nolint: errcheck
}

func (t *Tangle) applicationsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := req.URL.Query()

	// Bad label input is rejected before anything is sent to ArgoCD: a
	// malformed or duplicated pair means the caller's filter isn't the one
	// they wrote, and answering it anyway costs a fan-out across every
	// configured instance to return a result set that doesn't match the
	// request.
	labels, err := parseLabels("labels", query.Get("labels"))
	if err != nil {
		t.Log.Error("Invalid label query parameter", "parameter", "labels", "error", err)
		t.respondError(w, http.StatusBadRequest, err)
		return
	}

	excludeLabels, err := parseLabels("excludeLabels", query.Get("excludeLabels"))
	if err != nil {
		t.Log.Error("Invalid label query parameter", "parameter", "excludeLabels", "error", err)
		t.respondError(w, http.StatusBadRequest, err)
		return
	}

	if err := conflictingLabels(labels, excludeLabels); err != nil {
		t.Log.Error("Contradictory label query parameters", "error", err)
		t.respondError(w, http.StatusBadRequest, err)
		return
	}

	t.Log.Info("Listing applications by labels", "labels", labels, "excludeLabels", excludeLabels)

	apiResults := []ArgoCDApplicationResults{}
	for name, argoCD := range t.ArgoCDs {
		queryResults, err := argoCD.ListApplicationsByLabels(req.Context(), labels, excludeLabels)
		if err != nil {
			t.Log.Error("Failed to list applications by labels", "argocd", name, "error", err)
			t.respondError(w, http.StatusInternalServerError, err)
			return
		}

		baseLink := fmt.Sprintf("%s://%s/applications", argoCD.GetScheme(), argoCD.GetUrl())
		if len(labels) > 0 {
			mergedLabels := []string{}
			for key, value := range labels {
				mergedLabels = append(mergedLabels, fmt.Sprintf("%s%%253D%s", key, value))
			}

			tags := strings.Join(mergedLabels, "%2C")

			baseLink = fmt.Sprintf("%s?labels=%s", baseLink, tags)
		}

		argoCDApplicationResult := ArgoCDApplicationResults{
			Name:         name,
			Link:         baseLink,
			Applications: []ApplicationLinks{},
		}

		for _, queryResult := range queryResults {
			argoCDApplicationResult.Applications = append(argoCDApplicationResult.Applications, ApplicationLinks{
				Name:       queryResult.Name,
				URL:        fmt.Sprintf("%s://%s/applications/%s/%s", argoCD.GetScheme(), argoCD.GetUrl(), queryResult.Namespace, queryResult.Name),
				Health:     string(queryResult.Health.Status),
				SyncStatus: string(queryResult.SyncStatus.Status),
				LiveRef:    queryResult.LiveRevision,
			})
		}

		apiResults = append(apiResults, argoCDApplicationResult)
	}

	response := ApplicationsResponse{Results: t.sortResults(apiResults)}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (t *Tangle) applicationManifestsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	argocdName := chi.URLParam(req, "argocd")
	applicationName := chi.URLParam(req, "name")

	var diffsRequest DiffsRequest
	if err := json.NewDecoder(req.Body).Decode(&diffsRequest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	generatedManifests, err := t.ArgoCDs[argocdName].GetManifests(req.Context(), applicationName, diffsRequest.LiveRef, diffsRequest.TargetRef)
	if err != nil {
		t.Log.Error("Failed to get manifests", "argocd", argocdName, "error", err)
		response := DiffsResponse{
			ManifestGenerationError: err.Error(),
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	live, _ := assembleManifests(generatedManifests.LiveManifests)
	target, _ := assembleManifests(generatedManifests.TargetManifests)
	diff, _ := diffManifests(*live, *target)
	response := DiffsResponse{
		LiveManifests:   *live,
		TargetManifests: *target,
		Diffs:           *diff,
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}
