package manifests

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
	"knative.dev/eventmesh-operator/pkg/utils"
	"knative.dev/pkg/logging"
)

// eventingParser parses eventing manifests
type eventingParser struct {
	crdLister        apiextensionsv1.CustomResourceDefinitionLister
	deploymentLister appsv1listers.DeploymentLister
}

// NewEventingParser creates a new eventing manifest parser
func NewEventingParser(crdLister apiextensionsv1.CustomResourceDefinitionLister, deploymentLister appsv1listers.DeploymentLister) Parser {
	return &eventingParser{
		crdLister:        crdLister,
		deploymentLister: deploymentLister,
	}
}

// Parse returns the configured manifests for eventing
func (p *eventingParser) Parse(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := &Manifests{}

	coreManifests, err := p.eventingCoreManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests.Append(coreManifests)

	imcManifests, err := p.eventingIMCManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing IMC manifests: %w", err)
	}
	manifests.Append(imcManifests)

	mtBrokerManifests, err := p.eventingMTBrokerManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing MT channel Broker manifests: %w", err)
	}
	manifests.Append(mtBrokerManifests)

	tlsManifests, err := p.eventingTLSManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing tls manifests: %w", err)
	}
	manifests.Append(tlsManifests)

	postInstallManifests, err := p.eventingPostInstallManifests(ctx, em, coreManifests)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing post-install manifests: %w", err)
	}
	manifests.Append(postInstallManifests)

	return manifests, nil
}

func (p *eventingParser) eventingCoreManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	coreManifests, err := p.loadEventingCoreManifests()
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests.AddToApply(coreManifests)

	manifests.AddTransformers(
		transform.EventingCoreLogging(em.Spec.LogLevel),
		transform.DefaultChannelImplementation(em.Spec.DefaultChannel),
		transform.DefaultBrokerClass(em.Spec.DefaultBroker, em.Spec.DefaultChannel),
		transform.FeatureFlags(em.Spec.Features),
		transform.ConfigMapOverride(em.Spec.Overrides.Config),
		transform.WorkloadsOverride(em.Spec.Overrides.Workloads))

	return &manifests, nil
}

func (p *eventingParser) eventingIMCManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	imcManifests, err := loadManifests("eventing-latest", "in-memory-channel.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing IMC manifests: %w", err)
	}

	// we install the manifests by default, but scale the deployments down when no IMC exists
	manifests.AddToApply(imcManifests)

	return &manifests, nil
}

func (p *eventingParser) eventingMTBrokerManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	mtBroker, err := loadManifests("eventing-latest", "mt-channel-broker.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing MT channel broker manifests: %w", err)
	}

	// we install the manifests by default, but scale the deployments down when no MTCB broker exists
	manifests.AddToApply(mtBroker)

	return &manifests, nil
}

func (p *eventingParser) eventingTLSManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	tlsManifests, err := loadManifests("eventing-latest", "eventing-tls-networking.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing TLS manifests: %w", err)
	}

	certManagerIsInstalled, err := utils.IsCertmanagerInstalled(p.crdLister)
	if err != nil {
		return nil, fmt.Errorf("failed to check if cert-manager is installed: %w", err)
	}

	features, err := em.Spec.GetFeatureFlags()
	if err != nil {
		return nil, fmt.Errorf("failed to load feature flags: %w", err)
	}

	if !features.IsDisabledTransportEncryption() {
		if certManagerIsInstalled {
			manifests.AddToApply(tlsManifests)
		} else {
			return nil, fmt.Errorf("%s is set to %s, but cert-manager is not installed", feature.TransportEncryption, features[feature.TransportEncryption])
		}
	} else {
		if certManagerIsInstalled {
			manifests.AddToDelete(tlsManifests)
		}
	}

	return &manifests, nil
}

func (p *eventingParser) eventingPostInstallManifests(ctx context.Context, em *v1alpha1.EventMesh, manifests *Manifests) (*Manifests, error) {
	logger := logging.FromContext(ctx)

	upgrade, err := p.isUpgrade(manifests.ToApply)
	if err != nil {
		return nil, fmt.Errorf("failed to check if this is an upgrade: %w", err)
	}

	if upgrade {
		logger.Debug("Adding post-install manifests because at least minor version of manifests are upgraded")
		// we only install post-install manifests on updates

		postInstallManifests, err := loadManifests("eventing-latest", "eventing-post-install.yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to load eventing post-install manifests: %w", err)
		}

		result := Manifests{}
		result.AddToPostInstall(postInstallManifests)

		return &result, nil
	}
	logger.Debug("Skipping to add post-install manifests, as no (at least) minor version upgrade is ongoing")

	return nil, nil
}

func (p *eventingParser) isUpgrade(manifests mf.Manifest) (bool, error) {
	manifestVersion, instanceVersion, err := getVersions(manifests, p.deploymentLister)
	if err != nil {
		if errors.IsNotFound(err) {
			// this indicates, that none of the deployments from the manifests exists already in the cluster
			return false, nil
		}

		return false, fmt.Errorf("failed to get versions: %w", err)
	}

	manifestVersion.Patch = 0
	instanceVersion.Patch = 0

	return instanceVersion.LessThan(*manifestVersion), nil
}

func (p *eventingParser) loadEventingCoreManifests() (mf.Manifest, error) {
	eventingCoreFiles := []string{
		"eventing-crds.yaml",
		"eventing-core.yaml",
	}

	return loadManifests("eventing-latest", eventingCoreFiles...)
}
