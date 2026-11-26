package tangle

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/ivanklee86/tangle/internal/argocd"
)

// diffsTotal counts diff requests by outcome, so a rise in applications that
// need a target repo, or a spike in errors, shows up on a dashboard.
var diffsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "tangle_diffs_total",
	Help: "Diff requests by ArgoCD and outcome (ok, notAffected, needsTargetRepo, error).",
}, []string{"argocd", "status"})

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
	// Every source of the application, in ArgoCD's order: one for a
	// single-source application.
	Sources []ApplicationSource `json:"sources"`
}

// ApplicationSource is one entry of an application's sources.
type ApplicationSource struct {
	Position       int64  `json:"position"`
	Kind           string `json:"kind"`
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path,omitempty"`
	Chart          string `json:"chart,omitempty"`
	Ref            string `json:"ref,omitempty"`
	TargetRevision string `json:"targetRevision"`
}

func applicationSources(sources []argocd.Source) []ApplicationSource {
	result := make([]ApplicationSource, 0, len(sources))
	for _, source := range sources {
		result = append(result, ApplicationSource{
			Position:       source.Position,
			Kind:           string(source.Kind),
			RepoURL:        source.RepoURL,
			Path:           source.Path,
			Chart:          source.Chart,
			Ref:            source.Ref,
			TargetRevision: source.TargetRevision,
		})
	}
	return result
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

// ConfigResponse carries the settings the web UI can only learn at runtime.
//
// The frontend is a static bundle baked into the image once and served by every
// deployment, so a build-time variable can't hold anything deployment-specific.
// Deliberately named for the general case rather than /api/domain: version or
// instance metadata would belong here too.
type ConfigResponse struct {
	// Public base URL of this Tangle, without a trailing slash, or empty when
	// none is configured — in which case the browser uses its own origin.
	Domain string `json:"domain"`
}

// DiffsRequest contains the git refs to compare
// swagger:model DiffsRequest
type DiffsRequest struct {
	// Current git ref to compare from
	// required: true
	// example: main
	LiveRef string `json:"liveRef"`

	// Target ref to compare to: a branch, a tag or a commit SHA.
	// required: true
	// example: feature-branch
	TargetRef string `json:"targetRef"`

	// Repos the target ref belongs to. The ref goes to every source in any of
	// them, and every other source stays at its own targetRevision. Any
	// spelling of a repo URL matches (https, ssh, with or without .git).
	// Leave it out to diff single-source applications as before.
	// example: ["https://github.com/org/values.git"]
	TargetRepos []string `json:"targetRepos,omitempty"`

	// With no targetRepos, let a multi-source application whose git sources
	// are all in one repo use that repo.
	InferTargetRepo bool `json:"inferTargetRepo,omitempty"`
}

// Diff statuses, beyond the ones argocd.DiffStatus plans.
const (
	diffStatusError = "error"
)

// DiffSource is one source of the application, with the revision each side of
// the diff used.
type DiffSource struct {
	// 1-based source position, as ArgoCD counts them.
	Position int64 `json:"position"`
	// git, helm or oci.
	Kind    string `json:"kind"`
	RepoURL string `json:"repoURL"`
	Path    string `json:"path,omitempty"`
	Chart   string `json:"chart,omitempty"`
	Ref     string `json:"ref,omitempty"`
	// The revision the live side used.
	LiveRevision string `json:"liveRevision"`
	// The revision the target side used: the target ref when moved is true.
	TargetRevision string `json:"targetRevision"`
	Moved          bool   `json:"moved"`
}

type DiffsResponse struct {
	LiveManifests           string `json:"liveManifests"`
	TargetManifests         string `json:"targetManifests"`
	Diffs                   string `json:"diffs"`
	ManifestGenerationError string `json:"manifestGenerationError"`

	// ok, notAffected (target repos were given and none of the sources use
	// them, so nothing was rendered), needsTargetRepo (the application has
	// several sources and the request didn't say which repo the target ref
	// belongs to; manifestGenerationError says so too) or error.
	Status string `json:"status"`
	// The application's sources and the revision each side used.
	Sources []DiffSource `json:"sources,omitempty"`
	// The normalized repos the target ref went to.
	TargetRepos []string `json:"targetRepos,omitempty"`
	// True when targetRepos was inferred rather than given.
	InferredTargetRepo bool `json:"inferredTargetRepo,omitempty"`
	// For needsTargetRepo: the application's git repos, normalized, in
	// source order.
	CandidateRepos []string `json:"candidateRepos,omitempty"`
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

func (t *Tangle) configHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := ConfigResponse{Domain: t.Config.Domain}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
				Sources:    applicationSources(queryResult.Sources),
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

	if err := validateTargetRepos(diffsRequest.TargetRepos); err != nil {
		t.Log.Error("Invalid target repos", "argocd", argocdName, "application", applicationName, "error", err)
		t.respondError(w, http.StatusBadRequest, err)
		return
	}

	generatedManifests, err := t.ArgoCDs[argocdName].GetManifests(req.Context(), argocd.ManifestsRequest{
		ApplicationName: applicationName,
		LiveRef:         diffsRequest.LiveRef,
		TargetRef:       diffsRequest.TargetRef,
		TargetRepos:     diffsRequest.TargetRepos,
		InferTargetRepo: diffsRequest.InferTargetRepo,
	})
	if err != nil {
		t.Log.Error("Failed to get manifests", "argocd", argocdName, "application", applicationName, "error", err)
		diffsTotal.WithLabelValues(argocdName, diffStatusError).Inc()
		t.writeDiffsResponse(w, DiffsResponse{
			ManifestGenerationError: err.Error(),
			Status:                  diffStatusError,
		})
		return
	}

	plan := generatedManifests.Plan
	t.Log.Info("Planned diff",
		"argocd", argocdName,
		"application", applicationName,
		"status", plan.Status,
		"multiSource", plan.MultiSource,
		"targetRepos", plan.TargetRepos,
		"inferredTargetRepo", plan.Inferred,
		"positions", plan.Positions(),
		"liveRevisions", plan.LiveRevisions(),
		"targetRevisions", plan.TargetRevisions(),
	)
	diffsTotal.WithLabelValues(argocdName, string(plan.Status)).Inc()

	response := DiffsResponse{
		Status:             string(plan.Status),
		Sources:            diffSources(plan),
		TargetRepos:        plan.TargetRepos,
		InferredTargetRepo: plan.Inferred,
		CandidateRepos:     plan.CandidateRepos,
	}

	switch plan.Status {
	case argocd.DiffStatusNeedsTargetRepo:
		// Also an error, so a CLI that predates the status field still fails
		// under --fail-on-error (ADR 0032) instead of passing on an empty diff.
		response.ManifestGenerationError = needsTargetRepoMessage(applicationName, plan.CandidateRepos)
	case argocd.DiffStatusOK:
		live, _ := assembleManifests(generatedManifests.LiveManifests)
		target, _ := assembleManifests(generatedManifests.TargetManifests)
		diff, _ := diffManifests(*live, *target)
		response.LiveManifests = *live
		response.TargetManifests = *target
		response.Diffs = *diff
	}

	t.writeDiffsResponse(w, response)
}

func (t *Tangle) writeDiffsResponse(w http.ResponseWriter, response DiffsResponse) {
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// validateTargetRepos rejects a blank entry. Dropping it would quietly turn
// `--target-repo ""` into "no target repos", which changes what every
// multi-source application reports.
func validateTargetRepos(repos []string) error {
	for _, repo := range repos {
		if strings.TrimSpace(repo) == "" {
			return errors.New("targetRepos: entries must not be blank")
		}
	}
	return nil
}

func needsTargetRepoMessage(applicationName string, candidates []string) string {
	switch len(candidates) {
	case 0:
		return fmt.Sprintf("needs target repo: %s has several sources and none of them is a git repo a git ref could apply to", applicationName)
	case 1:
		return fmt.Sprintf("needs target repo: %s has several sources, with its git sources in %s. Pass that repo with --target-repo (targetRepos in the API), or turn on --infer-target-repo", applicationName, candidates[0])
	default:
		return fmt.Sprintf("needs target repo: %s has git sources in %d repos (%s). Pass the one the target ref belongs to with --target-repo (targetRepos in the API)", applicationName, len(candidates), strings.Join(candidates, ", "))
	}
}

func diffSources(plan argocd.RevisionPlan) []DiffSource {
	if len(plan.Sources) == 0 {
		return nil
	}

	sources := make([]DiffSource, 0, len(plan.Sources))
	for _, source := range plan.Sources {
		sources = append(sources, DiffSource{
			Position:       source.Position,
			Kind:           string(source.Kind),
			RepoURL:        source.RepoURL,
			Path:           source.Path,
			Chart:          source.Chart,
			Ref:            source.Ref,
			LiveRevision:   source.LiveRevision,
			TargetRevision: source.TargetRevision,
			Moved:          source.Moved,
		})
	}
	return sources
}
