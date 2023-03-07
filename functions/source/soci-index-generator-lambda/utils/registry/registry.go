// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"

	"github.com/aws-ia/cfn-aws-soci-index-builder/soci-index-generator-lambda/utils/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	MediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeDockerManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeOCIManifest        = "application/vnd.oci.image.manifest.v1+json"
)

type Registry struct {
	registry *remote.Registry
}

var RegistryNotSupportingOciArtifacts = errors.New("Registry does not support OCI artifacts")

// Initialize a remote registry
func Init(ctx context.Context, registryUrl string) (*Registry, error) {
	log.Info(ctx, "Initializing registry client")
	registry, err := remote.NewRegistry(registryUrl)
	if err != nil {
		return nil, err
	}
	if isEcrRegistry(registryUrl) {
		err := authorizeEcr(registry)
		if err != nil {
			return nil, err
		}
	}
	return &Registry{registry}, nil
}

// Pull an image from the remote registry to a local OCI Store
// imageReference can be either a digest or a tag
func (registry *Registry) Pull(ctx context.Context, repositoryName string, ociStore *oci.Store, imageReference string) (*ocispec.Descriptor, error) {
	log.Info(ctx, "Pulling image")
	repo, err := registry.registry.Repository(ctx, repositoryName)
	if err != nil {
		return nil, err
	}

	imageDescriptor, err := oras.Copy(ctx, repo, imageReference, ociStore, imageReference, oras.DefaultCopyOptions)
	if err != nil {
		return nil, err
	}

	return &imageDescriptor, nil
}

// Push a OCI artifact to remote registry
// descriptor: ocispec Descriptor of the artifact
// ociStore: the local OCI store
func (registry *Registry) Push(ctx context.Context, ociStore *oci.Store, indexDesc ocispec.Descriptor, repositoryName string) error {
	log.Info(ctx, "Pushing artifact")

	repo, err := registry.registry.Repository(ctx, repositoryName)
	if err != nil {
		return err
	}

	err = oras.CopyGraph(ctx, ociStore, repo, indexDesc, oras.DefaultCopyGraphOptions)
	if err != nil {
		// TODO: There might be a better way to check if a registry supporting OCI or not
		if strings.Contains(err.Error(), "Response status code 405: unsupported: Invalid parameter at 'ImageManifest' failed to satisfy constraint: 'Invalid JSON syntax'") {
			log.Warn(ctx, fmt.Sprintf("Error when pushing: %v", err))
			return RegistryNotSupportingOciArtifacts
		}
		return err
	}
	return nil
}

// Fetch the media type of an artifact
func (registry *Registry) GetMediaType(ctx context.Context, repositoryName string, reference string) (string, error) {
	repo, err := registry.registry.Repository(ctx, repositoryName)
	if err != nil {
		return "", err
	}

	descriptor, err := repo.Resolve(ctx, reference)
	if err != nil {
		return "", err
	}

	return descriptor.MediaType, nil
}

// Check if a registry is an ECR registry
func isEcrRegistry(registryUrl string) bool {
	ecrRegistryUrlRegex := "\\d{12}\\.dkr\\.ecr\\.\\S+\\.amazonaws\\.com"
	match, err := regexp.MatchString(ecrRegistryUrlRegex, registryUrl)
	if err != nil {
		panic(err)
	}
	return match
}

// Authorize ECR registry
func authorizeEcr(ecrRegistry *remote.Registry) error {
	// getting ecr auth token
	input := &ecr.GetAuthorizationTokenInput{}
	var ecrClient *ecr.ECR
	ecrEndpoint := os.Getenv("ECR_ENDPOINT") // set this env var for custom, i.e. non default, aws ecr endpoint
	if ecrEndpoint != "" {
		ecrClient = ecr.New(session.New(&aws.Config{Endpoint: aws.String(ecrEndpoint)}))
	} else {
		ecrClient = ecr.New(session.New())
	}
	getAuthorizationTokenResponse, err := ecrClient.GetAuthorizationToken(input)
	if err != nil {
		return err
	}

	if len(getAuthorizationTokenResponse.AuthorizationData) == 0 {
		return errors.New("Couldn't authorize with ECR: empty authorization data returned")
	}

	ecrAuthorizationToken := getAuthorizationTokenResponse.AuthorizationData[0].AuthorizationToken
	if len(*ecrAuthorizationToken) == 0 {
		return errors.New("Couldn't authorize with ECR: empty authorization token returned")
	}

	ecrRegistry.RepositoryOptions.Client = &auth.Client{
		Header: http.Header{
			"Authorization": {"Basic " + *ecrAuthorizationToken},
			"User-Agent":    {"SOCI Index Builder (oras-go)"},
		},
	}
	return nil
}