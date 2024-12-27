package tangle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ApplicationLinks struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type ArgoCDApplicationResults struct {
	Name         string             `json:"name"`
	Applications []ApplicationLinks `json:"applications"`
}

type ApplicationsResponse struct {
	Results []ArgoCDApplicationResults `json:"results"`
}

type ErrorResponse struct {
	Error string `json:"error"`
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

		argoCDApplicationResult := ArgoCDApplicationResults{
			Name:         name,
			Applications: []ApplicationLinks{},
		}

		for _, queryResult := range queryResults {
			argoCDApplicationResult.Applications = append(argoCDApplicationResult.Applications, ApplicationLinks{
				Name: queryResult.Name,
				URL:  fmt.Sprintf("https://%s/applications/%s/%s", argoCD.GetUrl(), queryResult.Namespace, queryResult.Name),
			})
		}

		apiResults = append(apiResults, argoCDApplicationResult)
	}

	response := ApplicationsResponse{Results: apiResults}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
