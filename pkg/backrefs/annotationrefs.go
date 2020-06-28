package backrefs

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ObjectFactory func() Object

type annotationRefs struct {
	ownerApiGroup string
}

func AnnotationReferences(ownerApiGroup string) BackReferenceStrategy {
	return &annotationRefs{ownerApiGroup}
}

func (s *annotationRefs) DelReference(from metav1.Object, to Object) bool {
	m := from.GetAnnotations()
	if m == nil {
		return false
	}
	a := s.annotation(to)
	if m[a] == "true" {
		delete(m, a)
		from.SetAnnotations(m)
		return true
	}
	return false
}

func (s *annotationRefs) AddReference(from metav1.Object, to Object) bool {
	m := from.GetAnnotations()
	if m == nil {
		m = map[string]string{}
	}
	a := s.annotation(to)
	if m[a] == "true" {
		return false
	}
	m[a] = "true"
	from.SetAnnotations(m)
	return true
}

func (s *annotationRefs) annotation(o Object) string {
	kind := o.GetObjectKind().GroupVersionKind().Kind
	checkKind(kind)
	ns := o.GetNamespace()
	name := o.GetName()
	return fmt.Sprintf("%s.%s/%s/%s", strings.ToLower(kind), s.ownerApiGroup, ns, name)
}
