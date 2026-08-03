package main

import (
	"reflect"
	"testing"
)

func TestShortOperationID(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	cases := []struct{ in, want string }{
		{"clientauth.v1.ClientCredentialService.MintClientToken", "MintClientToken"},
		{"images.v1.ImageService.ListImages", "ListImages"},
		{"videos.v1.VideoService.ListImages", "VideoServiceListImages"},
		{"ListImages", "ListImages2"},
	}
	for _, tc := range cases {
		if got := shortOperationID(tc.in, seen); got != tc.want {
			t.Errorf("shortOperationID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPackageList(t *testing.T) {
	t.Parallel()

	packages, err := packageList(map[string]bool{
		"Client Credentials": true,
		"One-Time Passcodes": true,
		"Images":             true,
	})
	if err != nil {
		t.Fatalf("packageList: %v", err)
	}

	want := []pkgTag{
		{pkg: "clientcredentials", tag: "Client Credentials"},
		{pkg: "images", tag: "Images"},
		{pkg: "onetimepasscodes", tag: "One-Time Passcodes"},
	}
	if !reflect.DeepEqual(packages, want) {
		t.Errorf("packageList = %+v, want %+v", packages, want)
	}
}

func TestPackageListRejectsCollidingTags(t *testing.T) {
	t.Parallel()

	if _, err := packageList(map[string]bool{"Api Keys": true, "API-Keys": true}); err == nil {
		t.Fatal("expected an error for tags collapsing to one package name")
	}
}

func TestFieldName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"Images":             "Images",
		"Client Credentials": "ClientCredentials",
		"One-Time Passcodes": "OneTimePasscodes",
	}
	for tag, want := range cases {
		if got := fieldName(tag); got != want {
			t.Errorf("fieldName(%q) = %q, want %q", tag, got, want)
		}
	}
}
