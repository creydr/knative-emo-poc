package transform

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConfigMapOverride(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]map[string]string
		input    *unstructured.Unstructured
		expected *unstructured.Unstructured
		wantErr  bool
	}{
		{
			name: "override configmap with exact name match",
			config: map[string]map[string]string{
				"test-config": {
					"key1": "value1",
					"key2": "value2",
				},
			},
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
					"data": map[string]interface{}{
						"key1": "oldvalue1",
						"key3": "value3",
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
					"data": map[string]interface{}{
						"key1": "value1",
						"key2": "value2",
						"key3": "value3",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "override configmap with config- prefix stripped",
			config: map[string]map[string]string{
				"test": {
					"key1": "value1",
				},
			},
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "config-test",
					},
					"data": map[string]interface{}{
						"key1": "oldvalue1",
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "config-test",
					},
					"data": map[string]interface{}{
						"key1": "value1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no override for non-matching configmap",
			config: map[string]map[string]string{
				"other-config": {
					"key1": "value1",
				},
			},
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
					"data": map[string]interface{}{
						"key1": "oldvalue1",
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
					"data": map[string]interface{}{
						"key1": "oldvalue1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no change for non-configmap resources",
			config: map[string]map[string]string{
				"test-config": {
					"key1": "value1",
				},
			},
			input: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Deployment",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
				},
			},
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Deployment",
					"metadata": map[string]interface{}{
						"name": "test-config",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transformer := ConfigMapOverride(test.config)
			err := transformer(test.input)

			if (err != nil) != test.wantErr {
				t.Errorf("ConfigMapOverride() error = %v, wantErr %v", err, test.wantErr)
				return
			}

			if diff := cmp.Diff(test.expected.Object, test.input.Object); diff != "" {
				t.Errorf("ConfigMapOverride() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
