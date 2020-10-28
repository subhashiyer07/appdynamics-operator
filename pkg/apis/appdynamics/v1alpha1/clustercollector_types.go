package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClustercollectorSpec defines the desired state of Clustercollector
type ClustercollectorSpec struct {
	Image              string                      `json:"image,omitempty"`
	ControllerUrl      string                      `json:"controllerUrl"`
	Account            string                      `json:"account,omitempty"`
	ServiceAccountName string                      `json:"serviceAccountName,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ClustercollectorStatus defines the observed state of Clustercollector
type ClustercollectorStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	State          AgentStatus `json:"state"`
}

// Clustercollector is the Schema for the clusteragents API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Clustercollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClustercollectorSpec   `json:"spec,omitempty"`
	Status ClustercollectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ClustercollectorList contains a list of Clustercollector
type ClustercollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Clustercollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Clustercollector{}, &ClustercollectorList{})
}
