// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

type ExpectedResponse struct {
	MediaType string
	Config    ocispec.Descriptor
}

func TestHeadManifest(t *testing.T) {
	doTest := func(registryUrl string, repository string, digestOrTag string, expected ExpectedResponse) {
		remote_registry, err := remote.NewRegistry(registryUrl)
		if err != nil {
			panic(err)
		}
		registry := &Registry{remote_registry}

		descriptor, err := registry.HeadManifest(context.Background(), repository, digestOrTag)
		if err != nil {
			panic(err)
		}
		if descriptor.MediaType != expected.MediaType {
			t.Fatalf("Incorrect manifest media type for %s/%s:%s. Expected %s but got %s", registryUrl, repository, digestOrTag, expected.MediaType, descriptor.MediaType)
		}
	}

	expected := ExpectedResponse{
		MediaType: MediaTypeOCIIndexManifest,
	}
	doTest("public.ecr.aws", "docker/library/redis", "7", expected)

	expected = ExpectedResponse{
		MediaType: MediaTypeDockerManifestList,
	}
	doTest("public.ecr.aws", "lambda/python", "3.10", expected)

	expected = ExpectedResponse{
		MediaType: MediaTypeDockerManifest,
	}
	doTest("public.ecr.aws", "lambda/python", "3.10-x86_64", expected)

	expected = ExpectedResponse{
		MediaType: MediaTypeDockerManifest,
	}
	doTest("docker.io", "library/redis", "sha256:afd1957d6b59bfff9615d7ec07001afb4eeea39eb341fc777c0caac3fcf52187", expected)
}

func TestGetManifest(t *testing.T) {
	doTest := func(registryUrl string, repository string, digestOrTag string, expected ExpectedResponse) {
		registry, err := Init(context.Background(), registryUrl, "")
		if err != nil {
			panic(err)
		}

		manifest, err := registry.GetManifest(context.Background(), repository, digestOrTag)
		if err != nil {
			panic(err)
		}
		if manifest.MediaType != expected.MediaType {
			t.Fatalf("Incorrect manifest media type for %s/%s:%s. Expected %s but got %s", registryUrl, repository, digestOrTag, expected.MediaType, manifest.MediaType)
		}

		if manifest.Config.MediaType != expected.Config.MediaType {
			t.Fatalf("Incorrect config's media type for %s/%s:%s. Expected %s but got %s", registryUrl, repository, digestOrTag, expected.Config.MediaType, manifest.Config.MediaType)
		}
	}

	expected := ExpectedResponse{
		MediaType: MediaTypeOCIIndexManifest,
		Config: ocispec.Descriptor{
			MediaType: "",
		},
	}
	doTest("public.ecr.aws", "docker/library/redis", "7", expected)

	expected = ExpectedResponse{
		MediaType: MediaTypeDockerManifestList,
		Config: ocispec.Descriptor{
			MediaType: "",
		},
	}
	doTest("public.ecr.aws", "lambda/python", "3.10", expected)

	expected = ExpectedResponse{
		MediaType: MediaTypeDockerManifest,
		Config: ocispec.Descriptor{
			MediaType: MediaTypeDockerImageConfig,
		},
	}
	doTest("public.ecr.aws", "lambda/python", "3.10-x86_64", expected)

	expected = ExpectedResponse{
		MediaType: MediaTypeDockerManifest,
		Config: ocispec.Descriptor{
			MediaType: MediaTypeDockerImageConfig,
		},
	}
	doTest("docker.io", "library/redis", "sha256:afd1957d6b59bfff9615d7ec07001afb4eeea39eb341fc777c0caac3fcf52187", expected)
}
