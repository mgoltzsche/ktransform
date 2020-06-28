package secrettransform

import (
	ktransformv1alpha1 "github.com/mgoltzsche/ktransform/pkg/apis/ktransform/v1alpha1"
	"github.com/mgoltzsche/ktransform/pkg/backrefs"
	corev1 "k8s.io/api/core/v1"
)

type referenceOwner struct {
	*ktransformv1alpha1.SecretTransform
}

func (s *referenceOwner) GetStatusReferences() []backrefs.Object {
	o := make([]backrefs.Object, 0, len(s.Status.ManagedReferences))
	for _, ref := range s.Status.ManagedReferences {
		switch ref.Kind {
		case "Secret":
			sec := &corev1.Secret{}
			sec.Name = ref.Name
			sec.Namespace = s.Namespace
			o = append(o, sec)
		case "ConfigMap":
			cm := &corev1.ConfigMap{}
			cm.Name = ref.Name
			cm.Namespace = s.Namespace
			o = append(o, cm)
		}
	}
	return o
}

func (s *referenceOwner) SetStatusReferences(refs []backrefs.Object) {
	o := make([]ktransformv1alpha1.ManagedReference, len(refs))
	for i, ref := range refs {
		o[i] = ktransformv1alpha1.ManagedReference{
			Kind: ref.GetObjectKind().GroupVersionKind().Kind,
			Name: ref.GetName(),
		}
	}
}

func (owner *referenceOwner) GetObject() backrefs.Object {
	return owner.SecretTransform
}
