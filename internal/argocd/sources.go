package argocd

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// SourceKind says how ArgoCD fetches a source, which decides whether a git ref
// can apply to it at all.
type SourceKind string

const (
	// SourceKindGit is a git repo: plain manifests, a chart by path, a CMP, or
	// a `ref` source other sources read values from.
	SourceKindGit SourceKind = "git"
	// SourceKindHelm is a chart from a Helm repository. Its targetRevision is a
	// chart version, never a git ref.
	SourceKindHelm SourceKind = "helm"
	// SourceKindOCI is an artifact from an OCI registry. Its targetRevision is
	// a tag or digest in the registry, never a git ref.
	SourceKindOCI SourceKind = "oci"
)

// Source is one entry of an Application's sources, flattened to what Tangle
// needs to pick revisions and to show the caller what was compared.
type Source struct {
	// Position is ArgoCD's 1-based source position, as SourcePositions uses.
	Position       int64
	RepoURL        string
	Path           string
	Chart          string
	Ref            string
	TargetRevision string
	Kind           SourceKind
}

// SourcesOf returns an Application's sources in ArgoCD's order. A single-source
// Application (spec.source) comes back as one source at position 1. An
// Application with neither returns nil.
func SourcesOf(app *v1alpha1.Application) []Source {
	if app == nil {
		return nil
	}

	var specSources []v1alpha1.ApplicationSource
	switch {
	case app.Spec.HasMultipleSources():
		specSources = app.Spec.Sources
	case app.Spec.Source != nil:
		specSources = []v1alpha1.ApplicationSource{*app.Spec.Source}
	default:
		return nil
	}

	sources := make([]Source, 0, len(specSources))
	for i, specSource := range specSources {
		sources = append(sources, Source{
			Position:       int64(i + 1),
			RepoURL:        specSource.RepoURL,
			Path:           specSource.Path,
			Chart:          specSource.Chart,
			Ref:            specSource.Ref,
			TargetRevision: specSource.TargetRevision,
			Kind:           kindOf(specSource),
		})
	}

	return sources
}

func kindOf(source v1alpha1.ApplicationSource) SourceKind {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.RepoURL)), "oci://") {
		return SourceKindOCI
	}

	if source.Chart != "" {
		return SourceKindHelm
	}

	return SourceKindGit
}

// scpLikeURL matches git's scp-style syntax, `user@host:path`, which has no
// scheme and so doesn't parse as a URL.
var scpLikeURL = regexp.MustCompile(`^(?:[^@/]+@)?([^/:]+):(.+)$`)

// NormalizeRepoURL reduces a git repo URL to `host/path`, so that one repo
// compares equal however it's spelled: https, ssh or scp-style, with or without
// a `.git` suffix, a trailing slash, credentials, a port or an upper-case host.
// The path keeps its case, because some git hosts treat it as significant.
func NormalizeRepoURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var host, path string
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Host == "" {
			return strings.ToLower(trimmed)
		}
		host = parsed.Hostname()
		path = parsed.Path
	} else if match := scpLikeURL.FindStringSubmatch(trimmed); match != nil {
		host = match[1]
		path = match[2]
	} else {
		// A bare `host/path`, as people tend to type into a form.
		host, path, _ = strings.Cut(trimmed, "/")
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")

	return strings.ToLower(host) + "/" + path
}
