package manifests

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

func TestGetVersions(t *testing.T) {
	manifestVersion := "1.2.3"
	instanceVersion := "1.2.2"

	tests := []struct {
		name             string
		manifests        mf.Manifest
		deployments      []*appsv1.Deployment
		expectedManifest *semver.Version
		expectedInstance *semver.Version
		wantErr          bool
	}{
		{
			name: "successful version retrieval",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("test-deployment", manifestVersion),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeployment("test-deployment", instanceVersion),
			},
			expectedManifest: semver.Must(semver.NewVersion(manifestVersion)),
			expectedInstance: semver.Must(semver.NewVersion(instanceVersion)),
			wantErr:          false,
		},
		{
			name: "multiple deployments - uses last one",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("first-deployment", "1.0.0"),
				createDeployment("last-deployment", manifestVersion),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeployment("first-deployment", "1.0.0"),
				createTypedDeployment("last-deployment", instanceVersion),
			},
			expectedManifest: semver.Must(semver.NewVersion(manifestVersion)),
			expectedInstance: semver.Must(semver.NewVersion(instanceVersion)),
			wantErr:          false,
		},
		{
			name: "deployment not found - tries next deployment",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("first-deployment", "1.0.0"),
				createDeployment("missing-deployment", manifestVersion),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeployment("first-deployment", instanceVersion),
			},
			expectedManifest: semver.Must(semver.NewVersion("1.0.0")),
			expectedInstance: semver.Must(semver.NewVersion(instanceVersion)),
			wantErr:          false,
		},
		{
			name: "no deployments in manifests",
			manifests: createManifest([]unstructured.Unstructured{
				createConfigMap("test-config"),
			}),
			deployments: []*appsv1.Deployment{},
			wantErr:     true,
		},
		{
			name: "deployment exists but missing version label",
			manifests: createManifest([]unstructured.Unstructured{
				createDeploymentWithoutVersion("test-deployment"),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeployment("test-deployment", instanceVersion),
			},
			wantErr: true,
		},
		{
			name: "running deployment missing version label",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("test-deployment", manifestVersion),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeploymentWithoutVersion("test-deployment"),
			},
			wantErr: true,
		},
		{
			name: "invalid version in manifest",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("test-deployment", "invalid-version"),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeployment("test-deployment", instanceVersion),
			},
			wantErr: true,
		},
		{
			name: "invalid version in running deployment",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("test-deployment", manifestVersion),
			}),
			deployments: []*appsv1.Deployment{
				createTypedDeployment("test-deployment", "invalid-version"),
			},
			wantErr: true,
		},
		{
			name: "no matching deployments found",
			manifests: createManifest([]unstructured.Unstructured{
				createDeployment("test-deployment", manifestVersion),
			}),
			deployments: []*appsv1.Deployment{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake deployment lister
			deploymentLister := createDeploymentLister(tt.deployments)

			manifestVersion, instanceVersion, err := getVersions(tt.manifests, deploymentLister)

			if tt.wantErr {
				if err == nil {
					t.Errorf("getVersions() expected error but got none")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("getVersions() unexpected error = %v", err)
				return
			}

			if diff := cmp.Diff(tt.expectedManifest, manifestVersion); diff != "" {
				t.Errorf("getVersions() manifest version mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tt.expectedInstance, instanceVersion); diff != "" {
				t.Errorf("getVersions() instance version mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Helper functions

func createManifest(resources []unstructured.Unstructured) mf.Manifest {
	manifest, _ := mf.ManifestFrom(mf.Slice(resources))
	return manifest
}

func createDeployment(name, version string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": system.Namespace(),
				"labels": map[string]interface{}{
					versionLabel: version,
				},
			},
		},
	}
}

func createDeploymentWithoutVersion(name string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": system.Namespace(),
			},
		},
	}
}

func createConfigMap(name string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": system.Namespace(),
			},
		},
	}
}

func createTypedDeployment(name, version string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: system.Namespace(),
			Labels: map[string]string{
				versionLabel: version,
			},
		},
	}
}

func createTypedDeploymentWithoutVersion(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: system.Namespace(),
		},
	}
}

func createDeploymentLister(deployments []*appsv1.Deployment) appsv1listers.DeploymentLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	for _, deployment := range deployments {
		_ = indexer.Add(deployment)
	}

	return appsv1listers.NewDeploymentLister(indexer)
}

func TestParseVersionFromLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected *semver.Version
		wantErr  bool
	}{
		{
			name: "valid version",
			labels: map[string]string{
				versionLabel: "1.2.3",
			},
			expected: semver.Must(semver.NewVersion("1.2.3")),
			wantErr:  false,
		},
		{
			name:    "missing version label",
			labels:  map[string]string{},
			wantErr: true,
		},
		{
			name: "invalid version format",
			labels: map[string]string{
				versionLabel: "invalid-version",
			},
			wantErr: true,
		},
		{
			name: "version with v prefix - should error",
			labels: map[string]string{
				versionLabel: "v1.2.3",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseVersionFromLabels(tt.labels)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseVersionFromLabels() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseVersionFromLabels() unexpected error = %v", err)
				return
			}

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("parseVersionFromLabels() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
