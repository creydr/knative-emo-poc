package utils

import (
	"fmt"

	apiextensionsv1listers "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func IsCertmanagerInstalled(crdLister apiextensionsv1listers.CustomResourceDefinitionLister) (bool, error) {
	_, err := crdLister.Get("certificates.cert-manager.io")
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get certificates.cert-manager.io CRD: %w", err)
	}

	return true, nil
}
