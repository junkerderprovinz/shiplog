package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sbomRegistry serves an image index for lib/app at sha256:index with an amd64
// and an arm64 image, an attestation for each, and the amd64 one's SPDX blob.
func sbomRegistry(t *testing.T, index string) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/lib/app/manifests/sha256:index", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
		_, _ = w.Write([]byte(index))
	})
	mux.HandleFunc("/v2/lib/app/manifests/sha256:att-amd64", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[
			{"mediaType":"application/vnd.in-toto+json","digest":"sha256:prov","annotations":{"in-toto.io/predicate-type":"https://slsa.dev/provenance/v1"}},
			{"mediaType":"application/vnd.in-toto+json","digest":"sha256:spdx","annotations":{"in-toto.io/predicate-type":"https://spdx.dev/Document"}}]}`))
	})
	mux.HandleFunc("/v2/lib/app/blobs/sha256:spdx", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"predicate":{"packages":[]}}`))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

const attestedIndex = `{"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[
	{"digest":"sha256:img-amd64","platform":{"os":"linux","architecture":"amd64"}},
	{"digest":"sha256:img-arm64","platform":{"os":"linux","architecture":"arm64","variant":"v8"}},
	{"digest":"sha256:att-amd64","platform":{"os":"unknown","architecture":"unknown"},
	 "annotations":{"vnd.docker.reference.type":"attestation-manifest","vnd.docker.reference.digest":"sha256:img-amd64"}},
	{"digest":"sha256:att-arm64","platform":{"os":"unknown","architecture":"unknown"},
	 "annotations":{"vnd.docker.reference.type":"attestation-manifest","vnd.docker.reference.digest":"sha256:img-arm64"}}]}`

func TestSBOMReadsTheAttestationOfThePlatformsImage(t *testing.T) {
	srv, paths := sbomRegistry(t, attestedIndex)
	doc, found, err := newResolverFor(srv).SBOM(context.Background(), "docker.io/lib/app", "sha256:index", "amd64")
	if err != nil || !found {
		t.Fatalf("SBOM = %v, %v", found, err)
	}
	if !strings.Contains(string(doc), `"predicate"`) {
		t.Errorf("doc = %s", doc)
	}
	got := strings.Join(*paths, ", ")
	want := "GET /v2/lib/app/manifests/sha256:index, GET /v2/lib/app/manifests/sha256:att-amd64, GET /v2/lib/app/blobs/sha256:spdx"
	if got != want {
		t.Errorf("requests = %s\nwant %s", got, want)
	}
}

func TestAnImageWithoutAttestationHasNoSBOM(t *testing.T) {
	cases := map[string]string{
		"no attestation": `{"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[
			{"digest":"sha256:img-amd64","platform":{"os":"linux","architecture":"amd64"}}]}`,
		"other platform only": `{"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[
			{"digest":"sha256:img-arm64","platform":{"os":"linux","architecture":"arm64"}}]}`,
		"single image": `{"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[]}`,
	}
	for name, index := range cases {
		srv, _ := sbomRegistry(t, index)
		_, found, err := newResolverFor(srv).SBOM(context.Background(), "docker.io/lib/app", "sha256:index", "amd64")
		if err != nil || found {
			t.Errorf("%s: SBOM = %v, %v; want none and no error", name, found, err)
		}
	}
}

func TestAnAttestationWithoutAnSPDXDocumentHasNoSBOM(t *testing.T) {
	srv, _ := sbomRegistry(t, attestedIndex)
	_, found, err := newResolverFor(srv).SBOM(context.Background(), "docker.io/lib/app", "sha256:index", "arm64")
	if err != nil || found {
		t.Errorf("SBOM = %v, %v; want none for an attestation the registry does not have", found, err)
	}
}

func TestARegistryErrorIsNotAMissingSBOM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, found, err := newResolverFor(srv).SBOM(context.Background(), "docker.io/lib/app", "sha256:index", "amd64")
	if err == nil || found {
		t.Errorf("SBOM = %v, %v; want an error", found, err)
	}
}
