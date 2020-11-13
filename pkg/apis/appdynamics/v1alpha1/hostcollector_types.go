package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HostcollectorSpec defines the desired state of Hostcollector
type HostcollectorSpec struct {
	Image              string                      `json:"image,omitempty"`
	ServiceAccountName string                      `json:"serviceAccountName,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
}

// HostcollectorStatus defines the observed state of Hostcollector
type HostcollectorStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	State          AgentStatus `json:"state"`
}

// Hostcollector is the Schema for the clusteragents API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Hostcollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostcollectorSpec   `json:"spec,omitempty"`
	Status HostcollectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// HostcollectorList contains a list of Hostcollector
type HostcollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Hostcollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Hostcollector{}, &HostcollectorList{})
}
