package manifests

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/api/errors"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/pkg/system"
)

const (
	versionLabel = "app.kubernetes.io/version"
)

func parseVersionFromLabels(labels map[string]string) (*semver.Version, error) {
	versionStr, ok := labels[versionLabel]
	if !ok {
		return nil, fmt.Errorf("could not get version from deployment labels. Label %s not found", versionLabel)
	}

	v, err := semver.NewVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse version from labels: %w", err)
	}

	return v, nil
}

// getVersions walks through the deployments from the given manifests (starting with the last one) and tries to get
// the version from the deployed resource and from the manifests. If the current object is not found in the cluster
// it tries the next one.
func getVersions(manifests mf.Manifest, deploymentLister appsv1listers.DeploymentLister) (*semver.Version, *semver.Version, error) {
	manifestDeployments := manifests.Filter(mf.ByKind("Deployment"))
	if len(manifestDeployments.Resources()) == 0 {
		return nil, nil, fmt.Errorf("could not find deployments in manifests")
	}
	lastDeployment := manifestDeployments.Resources()[len(manifestDeployments.Resources())-1]

	// get version from running deployment
	instance, err := deploymentLister.Deployments(system.Namespace()).Get(lastDeployment.GetName())
	if err != nil {
		// check if it is a not-found error and if we have any others deployments to check.
		if errors.IsNotFound(err) && len(manifestDeployments.Resources()) > 1 {
			// it could be a new resource added in the new release. Try next one...
			return getVersions(manifests.Filter(mf.Not(mf.All(mf.ByKind("Deployment"), mf.ByName(lastDeployment.GetName())))), deploymentLister)
		}

		return nil, nil, fmt.Errorf("could not get %s deployment: %w", lastDeployment.GetName(), err)
	}

	instanceVersion, err := parseVersionFromLabels(instance.Labels)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse version from deployment %s: %w", instance.Name, err)
	}

	// get version from last deployment in manifest list
	manifestVersion, err := parseVersionFromLabels(lastDeployment.GetLabels())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse version from manifest from %s: %w", lastDeployment.GetName(), err)
	}

	return manifestVersion, instanceVersion, nil
}
