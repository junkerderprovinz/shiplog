package vulns

import (
	"reflect"
	"strings"
	"testing"
)

func TestAPackageURLNamesWhatTheDatabaseKnows(t *testing.T) {
	cases := []struct {
		purl string
		want Package
	}{
		{"pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12&upstream=openssl", Package{"Debian:12", "openssl", "3.0.15-1~deb12u1"}},
		{"pkg:deb/debian/openssl@3.5.7-1~deb13u3?os_distro=trixie&os_name=debian&os_version=13", Package{"Debian:13", "openssl", "3.5.7-1~deb13u3"}},
		{"pkg:deb/debian/zlib1g@1%3A1.3.dfsg%2Breally1.3.1-1%2Bb1?arch=amd64&distro=debian-13&upstream=zlib%401%3A1.3.dfsg%2Breally1.3.1-1", Package{"Debian:13", "zlib", "1:1.3.dfsg+really1.3.1-1"}},
		{"pkg:deb/ubuntu/libc6@2.39-0ubuntu8.4?arch=amd64&distro=ubuntu-24.04&upstream=glibc", Package{"Ubuntu:24.04:LTS", "glibc", "2.39-0ubuntu8.4"}},
		{"pkg:deb/ubuntu/bash@5.2.32-1ubuntu1?arch=amd64&distro=ubuntu-25.10", Package{"Ubuntu:25.10", "bash", "5.2.32-1ubuntu1"}},
		{"pkg:apk/alpine/acl-libs@2.3.2-r1?arch=x86_64&distro=alpine-3.24.4&upstream=acl", Package{"Alpine:v3.24", "acl", "2.3.2-r1"}},
		{"pkg:npm/%40babel/traverse@7.23.0", Package{"npm", "@babel/traverse", "7.23.0"}},
		{"pkg:pypi/requests@2.31.0", Package{"PyPI", "requests", "2.31.0"}},
		{"pkg:golang/github.com/moby/sys@v0.1.0#user", Package{"Go", "github.com/moby/sys/user", "v0.1.0"}},
		{"pkg:golang/stdlib@go1.22.1", Package{"Go", "stdlib", "1.22.1"}},
		{"pkg:maven/org.apache.logging.log4j/log4j-core@2.14.0", Package{"Maven", "org.apache.logging.log4j:log4j-core", "2.14.0"}},
		{"pkg:nuget/BouncyCastle.Cryptography@2.6.2", Package{"NuGet", "BouncyCastle.Cryptography", "2.6.2"}},
		{"pkg:cargo/hyper@0.14.10", Package{"crates.io", "hyper", "0.14.10"}},
		{"pkg:gem/rack@2.2.3", Package{"RubyGems", "rack", "2.2.3"}},
		{"pkg:composer/symfony/http-kernel@5.4.0", Package{"Packagist", "symfony/http-kernel", "5.4.0"}},
	}
	for _, c := range cases {
		got, ok := fromPURL(c.purl)
		if !ok || got != c.want {
			t.Errorf("fromPURL(%s) = %+v, %v; want %+v", c.purl, got, ok, c.want)
		}
	}
}

func TestAPackageURLWithoutAKnownHomeIsLeftOut(t *testing.T) {
	for _, p := range []string{
		"pkg:rpm/redhat/openssl@3.0.7-1.el9",
		"pkg:deb/debian/bash@5.2?distro=debian-unstable",
		"pkg:deb/debian/bash",
		"pkg:generic/busybox@1.36",
		"not a purl",
	} {
		if got, ok := fromPURL(p); ok {
			t.Errorf("fromPURL(%s) = %+v, want it left out", p, got)
		}
	}
}

func TestTheReleaseFileNamesTheDistribution(t *testing.T) {
	cases := map[string]string{
		"PRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"\nID=debian\nVERSION_ID=\"12\"\n":                "Debian:12",
		"NAME=\"Ubuntu\"\nVERSION=\"22.04.5 LTS (Jammy Jellyfish)\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n": "Ubuntu:22.04:LTS",
		"ID=ubuntu\nVERSION_ID=\"25.10\"\n":                     "Ubuntu:25.10",
		"NAME=\"Alpine Linux\"\nID=alpine\nVERSION_ID=3.20.3\n": "Alpine:v3.20",
		"ID=debian\n":                        "",
		"ID=arch\nVERSION_ID=rolling\n":      "",
		"ID=\"rocky\"\nVERSION_ID=\"9.4\"\n": "",
	}
	for in, want := range cases {
		if got := ecosystem(in); got != want {
			t.Errorf("ecosystem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTheDpkgDatabaseListsInstalledSourcePackages(t *testing.T) {
	status := `Package: libssl3
Status: install ok installed
Source: openssl
Version: 3.0.15-1~deb12u1

Package: libc6
Status: install ok installed
Source: glibc (2.36-9+deb12u7)
Version: 2.36-9+deb12u7+b1

Package: bash
Status: install ok installed
Version: 5.2.15-2+b7

Package: removed-thing
Status: deinstall ok config-files
Version: 1.0

Package: openssl
Status: install ok installed
Version: 3.0.15-1~deb12u1
`
	got := parseDpkg(strings.NewReader(status), "Debian:12")
	want := []Package{
		{"Debian:12", "openssl", "3.0.15-1~deb12u1"},
		{"Debian:12", "glibc", "2.36-9+deb12u7"},
		{"Debian:12", "bash", "5.2.15-2+b7"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDpkg = %+v\nwant %+v", got, want)
	}
}

func TestADistrolessStatusFileCarriesNoStatusLine(t *testing.T) {
	got := parseDpkg(strings.NewReader("Package: tzdata\nVersion: 2024a-0+deb12u1\n"), "Debian:12")
	want := []Package{{"Debian:12", "tzdata", "2024a-0+deb12u1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDpkg = %+v, want %+v", got, want)
	}
}

func TestTheApkDatabaseListsOrigins(t *testing.T) {
	db := "C:Q1abc=\nP:libcrypto3\nV:3.3.2-r0\no:openssl\n\nP:musl\nV:1.2.5-r0\no:musl\n\nP:libssl3\nV:3.3.2-r0\no:openssl\n\nP:busybox\nV:1.36.1-r29\n"
	got := parseApk(strings.NewReader(db), "Alpine:v3.20")
	want := []Package{
		{"Alpine:v3.20", "openssl", "3.3.2-r0"},
		{"Alpine:v3.20", "musl", "1.2.5-r0"},
		{"Alpine:v3.20", "busybox", "1.36.1-r29"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseApk = %+v\nwant %+v", got, want)
	}
}

func TestAnAttestationListsThePackagesOfItsDocument(t *testing.T) {
	statement := `{"_type":"https://in-toto.io/Statement/v0.1","predicateType":"https://spdx.dev/Document","predicate":{
		"spdxVersion":"SPDX-2.3","packages":[
			{"name":"libssl3","externalRefs":[{"referenceCategory":"PACKAGE-MANAGER","referenceType":"purl","referenceLocator":"pkg:deb/debian/libssl3@3.0.15-1~deb12u1?distro=debian-12&upstream=openssl"}]},
			{"name":"openssl","externalRefs":[{"referenceType":"purl","referenceLocator":"pkg:deb/debian/openssl@3.0.15-1~deb12u1?distro=debian-12"}]},
			{"name":"rpm thing","externalRefs":[{"referenceType":"purl","referenceLocator":"pkg:rpm/fedora/x@1"}]},
			{"name":"no refs"}
		],
		"files":[{"fileName":"/etc/passwd"}]}}`
	got, err := parseSPDX([]byte(statement))
	if err != nil {
		t.Fatal(err)
	}
	want := []Package{{"Debian:12", "openssl", "3.0.15-1~deb12u1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSPDX = %+v, want %+v", got, want)
	}
}

func TestABareDocumentReadsLikeAnAttestation(t *testing.T) {
	doc := `{"spdxVersion":"SPDX-2.3","packages":[{"name":"requests","externalRefs":[{"referenceType":"purl","referenceLocator":"pkg:pypi/requests@2.31.0"}]}]}`
	got, err := parseSPDX([]byte(doc))
	if err != nil || len(got) != 1 || got[0].Name != "requests" {
		t.Errorf("parseSPDX = %+v, %v", got, err)
	}
}
