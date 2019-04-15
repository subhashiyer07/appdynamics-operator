package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterAgentSpec defines the desired state of ClusterAgent
type ClusterAgentSpec struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	ControllerUrl              string                      `json: "controllerUrl"`
	AccountName                string                      `json: "accountName,omitempty"`
	GlobalAccountName          string                      `json: "globalAccountName,omitempty"`
	EventServiceUrl            string                      `json: "eventServiceUrl,omitempty"`
	Image                      string                      `json: "image,omitempty"`
	Args                       []string                    `json: "args,omitempty"`
	Env                        []corev1.EnvVar             `json: "env,omitempty"`
	Resources                  corev1.ResourceRequirements `json: "resources,omitempty"`
	AgentName                  string                      `json: "agentName,omitempty"`
	AgentServerPort            int                         `json: "agentServerPort,omitempty"`
	SystemSSLCert              string                      `json: "systemSSLCert,omitempty"`
	AgentSSLCert               string                      `json: "agentSSLCert,omitempty"`
	EventAPILimit              int                         `json: "eventAPILimit,omitempty"`
	MetricsSyncInterval        int                         `json: "metricsSyncInterval,omitempty"`
	SnapshotSyncInterval       int                         `json: "snapshotSyncInterval,omitempty"`
	LogLines                   int                         `json: "logLines,omitempty"`
	LogLevel                   int                         `json: "logLevel,omitempty"`
	PodEventNumber             int                         `json: "podEventNumber,omitempty"`
	OverconsumptionThreshold   int                         `json: "overconsumptionThreshold,omitempty"`
	InstrumentationMethod      string                      `json: "instrumentationMethod,omitempty"`
	InstrumentMatchString      []string                    `json: "instrumentMatchString,omitempty"`
	DefaultInstrumentationTech string                      `json: "defaultInstrumentationTech,omitempty"`
	NsToInstrument             []string                    `json: "nsToInstrument,omitempty"`
	NsToInstrumentExclude      []string                    `json: "nsToInstrumentExclude,omitempty"`
	NsToMonitor                []string                    `json: "nsToMonitor,omitempty"`
	NsToMonitorExclude         []string                    `json: "nsToMonitorExclude,omitempty"`
	NodesToMonitor             []string                    `json: "nodesToMonitor,omitempty"`
	NodesToMonitorExclude      []string                    `json: "nodesToMonitorExclude,omitempty"`
	InstrumentRule             []AgentRequest              `json: "instrumentRule,omitempty"`
	AnalyticsAgentImage        string                      `json: "analyticsAgentImage,omitempty"`
	AppDJavaAttachImage        string                      `json: "appDJavaAttachImage,omitempty"`
	AppDDotNetAttachImage      string                      `json: "appDDotNetAttachImage,omitempty"`
	BiqService                 string                      `json: "biqService,omitempty"`
	InstrumentContainer        string                      `json: "instrumentContainer,omitempty"`

	InitContainerDir            string `json: "initContainerDir,omitempty"`
	AgentLabel                  string `json: "agentLabel,omitempty"`
	AgentEnvVar                 string `json: "agentEnvVar,omitempty"`
	AppDAppLabel                string `json: "appDAppLabel,omitempty"`
	AppDTierLabel               string `json: "appDTierLabel,omitempty"`
	AppDAnalyticsLabel          string `json: "appDAnalyticsLabel,omitempty"`
	AgentMountName              string `json: "agentMountName,omitempty"`
	AgentMountPath              string `json: "agentMountPath,omitempty"`
	AppLogMountName             string `json: "appLogMountName,omitempty"`
	AppLogMountPath             string `json: "appLogMountPath,omitempty"`
	JDKMountName                string `json: "jDKMountName,omitempty"`
	JDKMountPath                string `json: "jDKMountPath,omitempty"`
	NodeNamePrefix              string `json: "nodeNamePrefix,omitempty"`
	AnalyticsAgentUrl           string `json: "analyticsAgentUrl,omitempty"`
	AnalyticsAgentContainerName string `json: "analyticsAgentContainerName,omitempty"`
	AppDInitContainerName       string `json: "appDInitContainerName,omitempty"`

	InitRequestMem string `json: "initRequestMem,omitempty"`
	InitRequestCpu string `json: "initRequestCpu,omitempty"`
	BiqRequestMem  string `json: "biqRequestMem,omitempty"`
	BiqRequestCpu  string `json: "biqRequestCpu,omitempty"`

	PodSchemaName       string `json: "podSchemaName,omitempty"`
	NodeSchemaName      string `json: "nodeSchemaName,omitempty"`
	EventSchemaName     string `json: "eventSchemaName,omitempty"`
	ContainerSchemaName string `json: "containerSchemaName,omitempty"`
	JobSchemaName       string `json: "jobSchemaName,omitempty"`
	LogSchemaName       string `json: "logSchemaName,omitempty"`
	EpSchemaName        string `json: "epSchemaName,omitempty"`
	NsSchemaName        string `json: "nsSchemaName,omitempty"`
	RqSchemaName        string `json: "rqSchemaName,omitempty"`
	DeploySchemaName    string `json: "deploySchemaName,omitempty"`
	RSSchemaName        string `json: "rSSchemaName,omitempty"`
	DaemonSchemaName    string `json: "daemonSchemaName,omitempty"`

	DashboardTiers        []string `json: "dashboardTiers,omitempty"`
	DashboardSuffix       string   `json: "dashboardSuffix,omitempty"`
	DashboardDelayMin     string   `json: "dashboardDelayMin,omitempty"`
	DashboardTemplatePath string   `json: "dashboardTemplatePath,omitempty"`

	ProxyUrl  string `json: "proxyUrl,omitempty"`
	ProxyUser string `json: "proxyUser,omitempty"`
	ProxyPass string `json: "proxyPass,omitempty"`
}

// ClusterAgentStatus defines the observed state of ClusterAgent
type ClusterAgentStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	LastUpdateTime metav1.Time `json: "lastMetricsUpdateTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAgent is the Schema for the clusteragents API
// +k8s:openapi-gen=true
type ClusterAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAgentSpec   `json:"spec,omitempty"`
	Status ClusterAgentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAgentList contains a list of ClusterAgent
type ClusterAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterAgent{}, &ClusterAgentList{})
}
