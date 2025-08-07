package manifests

import (
	"fmt"
	"os"

	mf "github.com/manifestival/manifestival"
)

type Manifests struct {
	ToApply      mf.Manifest
	ToDelete     mf.Manifest
	Transformers []mf.Transformer
}

func (m *Manifests) AddToApply(manifests mf.Manifest) {
	m.ToApply = m.ToApply.Append(manifests)
}

func (m *Manifests) AddToDelete(manifests mf.Manifest) {
	m.ToDelete = m.ToDelete.Append(manifests)
}

func (m *Manifests) AddTransformers(transformers ...mf.Transformer) {
	m.Transformers = append(m.Transformers, transformers...)
}

func (m *Manifests) TransformToApply() error {
	patched, err := m.ToApply.Transform(m.Transformers...)
	if err != nil {
		return fmt.Errorf("failed to transform eventing manifests to apply: %w", err)
	}
	m.ToApply = patched

	return nil
}

func (m *Manifests) TransformToDelete() error {
	patched, err := m.ToDelete.Transform(m.Transformers...)
	if err != nil {
		return fmt.Errorf("failed to transform eventing manifests to delete: %w", err)
	}
	m.ToDelete = patched

	return nil
}

func (m *Manifests) Sort() {
	m.ToApply = m.ToApply.Sort(mf.ByKindPriority())
	m.ToDelete = m.ToDelete.Sort(mf.ByKindPriority()) //sort it here. m.Delete() will delete in reverse order
}

func (m *Manifests) Append(manifests *Manifests) {
	m.AddToApply(manifests.ToApply)
	m.AddToDelete(manifests.ToDelete)
	m.AddTransformers(manifests.Transformers...)
}

func loadManifests(dirname string, filenames ...string) (mf.Manifest, error) {
	manifests := mf.Manifest{}
	for _, file := range filenames {
		manifest, err := mf.NewManifest(fmt.Sprintf("%s/%s/%s", os.Getenv("KO_DATA_PATH"), dirname, file))
		if err != nil {
			return mf.Manifest{}, fmt.Errorf("failed to parse manifest %s/%s: %w", dirname, file, err)
		}

		manifests = manifests.Append(manifest)
	}

	return manifests, nil
}
