package tangle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ApplicationLinks struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Health         string `json:"health"`
	SyncStatus     string `json:"syncStatus"`
	TargetRevision string `json:"targetRevision"`
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

func (t *Tangle) applicationsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := req.URL.Query()

	labels := make(map[string]string)
	if len(query.Get("labels")) > 0 {
		rawLabels := strings.Split(query.Get("labels"), ",")
		for idx := range rawLabels {
			rawLabel := strings.Split(rawLabels[idx], ":")
			labels[rawLabel[0]] = rawLabel[1]
		}
	}

	apiResults := []ArgoCDApplicationResults{}
	for name, argoCD := range t.ArgoCDs {
		queryResults, err := argoCD.ListApplicationsByLabels(req.Context(), labels)
		if err != nil {
			t.Log.Error("Failed to list applications by labels", "argocd", name, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()}) // nolint: errcheck
			return
		}

		baseLink := fmt.Sprintf("https://%s/applications", argoCD.GetUrl())
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
				Name:           queryResult.Name,
				URL:            fmt.Sprintf("https://%s/applications/%s/%s", argoCD.GetUrl(), queryResult.Namespace, queryResult.Name),
				Health:         string(queryResult.Health.Status),
				SyncStatus:     string(queryResult.SyncStatus.Status),
				TargetRevision: queryResult.TargetRevision,
			})
		}

		apiResults = append(apiResults, argoCDApplicationResult)
	}

	response := ApplicationsResponse{Results: t.sortResults(apiResults)}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
