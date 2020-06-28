package v1alpha1

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionSynced       = "Synced"
	ReasonMissingInput    = status.ConditionReason("MissingInput")
	ReasonInvalidSpec     = status.ConditionReason("InvalidSpec")
	ReasonFailedTransform = status.ConditionReason("FailedTransform")
	ReasonFailedWrite     = status.ConditionReason("FailedWrite")
	ReasonFailed          = status.ConditionReason("Failed")
)

// SecretTransformSpec defines the desired state of SecretTransform
type SecretTransformSpec struct {
	Input  map[string]InputRef `json:"input,omitempty"`
	Output []Output            `json:"output"`
}

type InputRef struct {
	Secret    *string `json:"secret,omitempty"`
	ConfigMap *string `json:"configMap,omitempty"`
}

type Output struct {
	Secret         *SecretOutput     `json:"secret,omitempty"`
	ConfigMap      *ConfigMapOutput  `json:"configMap,omitempty"`
	Transformation map[string]string `json:"transformation,omitempty"`
}

type SecretOutput struct {
	Name string            `json:"name"`
	Type corev1.SecretType `json:"type,omitempty"`
}

type ConfigMapOutput struct {
	Name string `json:"name"`
}

// SecretTransformStatus defines the observed state of SecretTransform
type SecretTransformStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         status.Conditions  `json:"conditions,omitempty"`
	ManagedReferences  []ManagedReference `json:"managedReferences,omitempty"`
	OutputHash         string             `json:"outputHash,omitempty"`
}

type ManagedReference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretTransform is the Schema for the secrettransforms API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=secrettransforms,scope=Namespaced
type SecretTransform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretTransformSpec   `json:"spec,omitempty"`
	Status SecretTransformStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretTransformList contains a list of SecretTransform
type SecretTransformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretTransform `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretTransform{}, &SecretTransformList{})
}
