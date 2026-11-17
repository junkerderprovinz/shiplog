package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	acceptManifest = "application/vnd.oci.image.manifest.v1+json"
	spdxPredicate  = "https://spdx.dev/Document"
	maxSBOM        = 64 << 20
)

// SBOM returns the SPDX document a build attached to the image behind digest
// for one architecture. BuildKit keeps it in the image index, as an
// attestation manifest that refers to the platform's image. found is false
// when the image has none, which never changes for a digest.
func (r *Resolver) SBOM(ctx context.Context, repo, digest, arch string) (doc []byte, found bool, err error) {
	host, pathRepo := splitRepo(repo)
	base := r.baseURL(host)
	if r.breakerRemaining(host) > 0 {
		return nil, false, fmt.Errorf("%s is rate limiting us", host)
	}

	var index struct {
		Manifests []struct {
			Digest   string `json:"digest"`
			Platform struct {
				OS           string `json:"os"`
				Architecture string `json:"architecture"`
			} `json:"platform"`
			Annotations map[string]string `json:"annotations"`
		} `json:"manifests"`
	}
	if ok, err := r.getJSON(ctx, base+"/v2/"+pathRepo+"/manifests/"+digest, host, pathRepo, acceptManifests, &index); !ok || err != nil {
		return nil, false, err
	}
	image := ""
	for _, m := range index.Manifests {
		if m.Platform.OS == "linux" && m.Platform.Architecture == arch {
			image = m.Digest
			break
		}
	}
	attestation := ""
	for _, m := range index.Manifests {
		if image != "" && m.Annotations["vnd.docker.reference.type"] == "attestation-manifest" && m.Annotations["vnd.docker.reference.digest"] == image {
			attestation = m.Digest
		}
	}
	if attestation == "" {
		return nil, false, nil
	}

	var manifest struct {
		Layers []struct {
			Digest      string            `json:"digest"`
			Annotations map[string]string `json:"annotations"`
		} `json:"layers"`
	}
	if ok, err := r.getJSON(ctx, base+"/v2/"+pathRepo+"/manifests/"+attestation, host, pathRepo, acceptManifest, &manifest); !ok || err != nil {
		return nil, false, err
	}
	for _, l := range manifest.Layers {
		if l.Annotations["in-toto.io/predicate-type"] != spdxPredicate {
			continue
		}
		resp, err := r.doAuthed(ctx, http.MethodGet, base+"/v2/"+pathRepo+"/blobs/"+l.Digest, host, pathRepo)
		if err != nil {
			return nil, false, err
		}
		defer func() { _ = resp.Body.Close() }()
		r.noteHostOutcome(host, resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			return nil, false, fmt.Errorf("blob %s: unexpected status %d", l.Digest, resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxSBOM))
		return b, err == nil, err
	}
	return nil, false, nil
}

// getJSON decodes a manifest. A 404 reports false without an error, since a
// registry drops an old index once nothing refers to it any more.
func (r *Resolver) getJSON(ctx context.Context, url, host, pathRepo, accept string, out any) (bool, error) {
	resp, err := r.doAuthed(ctx, http.MethodGet, url, host, pathRepo, header{"Accept", accept})
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	r.noteHostOutcome(host, resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("manifest: unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
		return false, fmt.Errorf("manifest: %w", err)
	}
	return true, nil
}
