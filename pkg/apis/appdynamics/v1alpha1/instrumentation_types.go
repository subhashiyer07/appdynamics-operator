package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusteragentSpec defines the desired state of Clusteragent
type InstrumentationSpec struct {
	ClusteragentSpec

	//instrumentation
	InstrumentationMethod        string                `json:"instrumentationMethod,omitempty"`
	InstrumentMatchString        []string              `json:"instrumentMatchString,omitempty"`
	DefaultInstrumentMatchString string                `json:"defaultInstrumentMatchString,omitempty"`
	DefaultInstrumentationTech   string                `json:"defaultInstrumentationTech,omitempty"`
	DefaultLabelMatch            map[string]string     `json:"defaultInstrumentionLabelMatch,omitempty"`
	NsToInstrument               []string              `json:"nsToInstrument,omitempty"`
	NsToInstrumentRegex          string                `json:"nsToInstrumentRegex,omitempty"`
	NsToInstrumentExclude        []string              `json:"nsToInstrumentExclude,omitempty"`
	NsToMonitor                  []string              `json:"nsToMonitor,omitempty"`
	NsToMonitorExclude           []string              `json:"nsToMonitorExclude,omitempty"`
	ResourcesToInstrument        []string              `json:"resourcesToInstrument,omitempty"`
	DefaultEnv                   string                `json:"defaultEnv,omitempty"`
	DefaultCustomConfig          string                `json:"defaultCustomConfig,omitempty"`
	DefaultAppName               string                `json:"defaultAppName,omitempty"`
	DefaultContainerMatchString  string                `json:"defaultContainerMatchString,omitempty"`
	NetvizInfo                   NetvizInfo            `json:"netvizInfo,omitempty"`
	NSRules                      []NSRule              `json:"namespaceRules,omitempty"`
	NodesToMonitor               []string              `json:"nodesToMonitor,omitempty"`
	NodesToMonitorExclude        []string              `json:"nodesToMonitorExclude,omitempty"`
	InstrumentRule               []AgentRequest        `json:"instrumentRule,omitempty"`
	AnalyticsAgentImage          string                `json:"analyticsAgentImage,omitempty"`
	AppDJavaAttachImage          string                `json:"appDJavaAttachImage,omitempty"`
	AppDDotNetAttachImage        string                `json:"appDDotNetAttachImage,omitempty"`
	BiqService                   string                `json:"biqService,omitempty"`
	InstrumentContainer          string                `json:"instrumentContainer,omitempty"`
	InitContainerDir             string                `json:"initContainerDir,omitempty"`
	AgentLabel                   string                `json:"agentLabel,omitempty"`
	AgentLogOverride             string                `json:"agentLogOverride,omitempty"`
	AgentUserOverride            string                `json:"agentUserOverride,omitempty"`
	AgentEnvVar                  string                `json:"agentEnvVar,omitempty"`
	AgentOpts                    string                `json:"agentOpts,omitempty"`
	AppNameLiteral               string                `json:"appNameLiteral,omitempty"`
	AppDAppLabel                 string                `json:"appDAppLabel,omitempty"`
	AppDTierLabel                string                `json:"appDTierLabel,omitempty"`
	AppDAnalyticsLabel           string                `json:"appDAnalyticsLabel,omitempty"`
	AgentMountName               string                `json:"agentMountName,omitempty"`
	AgentMountPath               string                `json:"agentMountPath,omitempty"`
	AppLogMountName              string                `json:"appLogMountName,omitempty"`
	AppLogMountPath              string                `json:"appLogMountPath,omitempty"`
	JDKMountName                 string                `json:"jDKMountName,omitempty"`
	JDKMountPath                 string                `json:"jDKMountPath,omitempty"`
	NodeNamePrefix               string                `json:"nodeNamePrefix,omitempty"`
	AnalyticsAgentUrl            string                `json:"analyticsAgentUrl,omitempty"`
	AnalyticsAgentContainerName  string                `json:"analyticsAgentContainerName,omitempty"`
	AppDInitContainerName        string                `json:"appDInitContainerName,omitempty"`
	NetVizPort                   int                   `json:"netVizPort,omitempty"`
	InitRequestMem               string                `json:"initRequestMem,omitempty"`
	InitRequestCpu               string                `json:"initRequestCpu,omitempty"`
	BiqRequestMem                string                `json:"biqRequestMem,omitempty"`
	BiqRequestCpu                string                `json:"biqRequestCpu,omitempty"`
	UniqueHostID                 string                `json:"uniqueHostID,omitempty"`
	AgentSSLStoreName            string                `json:"agentSSLStoreName,omitempty"`
	AgentSSLPassword             string                `json:"agentSSLPassword,omitempty"`
	PodFilter                    ClusteragentPodFilter `json:"podFilter,omitempty"`
	ImageInfoMap                 map[string]ImageInfo  `json:"image-info,omitempty"`
}

// ClusteragentStatus defines the observed state of Clusteragent
type InstrumentationStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	State          AgentStatus `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Clusteragent is the Schema for the clusteragents API
// +k8s:openapi-gen=true
type InstrumentationAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstrumentationSpec   `json:"spec,omitempty"`
	Status InstrumentationStatus `json:"status,omitempty"`
}

type NetvizInfo struct {
	BciEnabled bool `json:"bciEnabled,omitempty"`
	Port       int  `json:"port,omitempty"`
}

type ImageInfo struct {
	Image          string `json:"image"`
	AgentMountPath string `json:"agentMountPath"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusteragentList contains a list of Clusteragent
type InstrumentationAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstrumentationAgent `json:"items"`
}

type NSRule struct {
	NamespaceRegex           string            `json:"namespaceRegex,omitempty"`
	MatchString              string            `json:"matchString,omitempty"`
	LabelMatch               map[string]string `json:"labelMatch,omitempty"`
	AppName                  string            `json:"appName,omitempty"`
	TierName                 string            `json:"tierName,omitempty"`
	Language                 string            `json:"language,omitempty"`
	InstrumentContainer      string            `json:"instrumentContainer,omitempty"`
	ContainerNameMatchString string            `json:"containerMatchString,omitempty"`
	CustomAgentConfig        string            `json:"customAgentConfig,omitempty"`
	EnvToUse                 string            `json:"env,omitempty"`
}

func init() {
	SchemeBuilder.Register(&InstrumentationAgent{}, &InstrumentationAgentList{})
}
