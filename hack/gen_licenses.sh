#!/usr/bin/env bash
# Modified by the kube-node-viewer authors, 2026.
go install github.com/google/go-licenses@latest
go mod download
# hack/ holds several main packages, so report only the shipped code.
GOROOT=$(go env GOROOT) "$(go env GOPATH)/bin/go-licenses" report ./cmd/... ./pkg/... --template hack/attribution.tmpl > ATTRIBUTION.md
