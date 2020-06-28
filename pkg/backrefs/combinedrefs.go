package backrefs

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type annotationOrOwnerRefs struct {
	annotationRefs
	ownerRefs
}

func AnnotationOrOwnerReferences(annotationPrefix string) BackReferenceStrategy {
	return &annotationOrOwnerRefs{annotationRefs{annotationPrefix}, ownerRefs{}}
}

func (s *annotationOrOwnerRefs) DelReference(from metav1.Object, to Object) bool {
	if from.GetNamespace() == to.GetNamespace() {
		return s.ownerRefs.DelReference(from, to)
	}
	return s.annotationRefs.DelReference(from, to)
}

func (s *annotationOrOwnerRefs) AddReference(from metav1.Object, to Object) bool {
	if from.GetNamespace() == to.GetNamespace() {
		return s.ownerRefs.AddReference(from, to)
	}
	return s.annotationRefs.AddReference(from, to)
}
