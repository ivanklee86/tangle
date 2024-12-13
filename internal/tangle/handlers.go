package tangle

import (
	"encoding/json"
	"net/http"
	"strings"
)

type ApplicationsResponse struct {
	Results []string `json:"results"`
}

func (t *Tangle) applicationsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	query := req.URL.Query()

	labels := make(map[string]string)
	rawLabels := strings.Split(query.Get("labels"), ",")
	for idx := range rawLabels {
		rawLabel := strings.Split(rawLabels[idx], ":")
		labels[rawLabel[0]] = rawLabel[1]
	}

	queryResults := t.ArgoCDClients["test"].ListApplicationsByLabels(labels)

	response := ApplicationsResponse{Results: queryResults}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
