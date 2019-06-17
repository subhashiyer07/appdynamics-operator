package v1alpha1

type TechnologyName string

const (
	Java   TechnologyName = "java"
	DotNet TechnologyName = "dotnet"
	NodeJS TechnologyName = "nodejs"
)

type InstrumentationMethod string

const (
	None        InstrumentationMethod = ""
	CopyAttach  InstrumentationMethod = "copy"
	MountAttach InstrumentationMethod = "mountAttach"
	MountEnv    InstrumentationMethod = "mountEnv"
)

type AgentRequest struct {
	Namespaces     []string              `json:"namespaces,omitempty"`
	AppDAppLabel   string                `json:"appDAppLabel,omitempty"`
	AppDTierLabel  string                `json:"appDTierLabel,omitempty"`
	Tech           TechnologyName        `json:"tech,omitempty"`
	ContainerName  string                `json:"containerName,omitempty"`
	Version        string                `json:"version,omitempty"`
	MatchString    []string              `json:"matchString,omitempty"` //string matched against deployment names and labels, supports regex
	Method         InstrumentationMethod `json:"method,omitempty"`
	BiQ            string                `json:"biQ,omitempty"` //"sidecar" or reference to the remote analytics agent
	AppNameLiteral string                `json:"appNameLiteral,omitempty"`
	AgentEnvVar    string                `json:"agentEnvVar,omitempty"`
}

type AgentStatus struct {
	Version                    string                `json:"version,omitempty"`
	MetricsSyncInterval        int                   `json:"metricsSyncInterval"`
	SnapshotSyncInterval       int                   `json:"snapshotSyncInterval"`
	LogLevel                   string                `json:"logLevel"`
	LogLines                   int                   `json:"logLines"`
	NsToMonitor                []string              `json:"nsToMonitor,omitempty"`
	NsToMonitorExclude         []string              `json:"nsToMonitorExclude,omitempty"`
	NodesToMonitor             []string              `json:"nodesToMonitor,omitempty"`
	NodesToMonitorExclude      []string              `json:"nodesToMonitorExclude,omitempty"`
	NsToInstrument             []string              `json:"nsToInstrument,omitempty"`
	NsToInstrumentExclude      []string              `json:"nsToInstrumentExclude,omitempty"`
	InstrumentRule             []AgentRequest        `json:"instrumentRule,omitempty"`
	InstrumentationMethod      InstrumentationMethod `json:"instrumentationMethod,omitempty"`
	DefaultInstrumentationTech TechnologyName        `json:"defaultInstrumentationTech,omitempty"`
	InstrumentMatchString      []string              `json:"instrumentMatchString,omitempty"`
	BiqService                 string                `json:"biqService,omitempty"`
	AnalyticsAgentImage        string                `json:"analyticsAgentImage,omitempty"`
	AppDJavaAttachImage        string                `json:"appDJavaAttachImage,omitempty"`
	AppDDotNetAttachImage      string                `json:"appDDotNetAttachImage,omitempty"`
}

type AppDBag struct {
	AgentNamespace              string
	AppName                     string
	TierName                    string
	NodeName                    string
	AppID                       int
	TierID                      int
	NodeID                      int
	Account                     string
	GlobalAccount               string
	AccessKey                   string
	ControllerUrl               string
	ControllerPort              uint16
	RestAPIUrl                  string
	SSLEnabled                  bool
	SystemSSLCert               string
	AgentSSLCert                string
	EventKey                    string
	EventServiceUrl             string
	RestAPICred                 string
	EventAPILimit               int
	PodSchemaName               string
	NodeSchemaName              string
	DeploySchemaName            string
	RSSchemaName                string
	DaemonSchemaName            string
	EventSchemaName             string
	ContainerSchemaName         string
	EpSchemaName                string
	NsSchemaName                string
	RqSchemaName                string
	JobSchemaName               string
	LogSchemaName               string
	DashboardTemplatePath       string
	DashboardSuffix             string
	DashboardDelayMin           int
	AgentEnvVar                 string
	AgentLabel                  string
	AgentLogOverride            string
	AgentUserOverride           string
	AppNameLiteral              string
	AppDAppLabel                string
	AppDTierLabel               string
	AppDAnalyticsLabel          string
	AgentMountName              string
	AgentMountPath              string
	AppLogMountName             string
	AppLogMountPath             string
	JDKMountName                string
	JDKMountPath                string
	NodeNamePrefix              string
	AnalyticsAgentUrl           string
	AnalyticsAgentImage         string
	AnalyticsAgentContainerName string
	AppDInitContainerName       string
	AppDJavaAttachImage         string
	AppDDotNetAttachImage       string
	AppDNodeJSAttachImage       string
	ProxyUrl                    string
	ProxyUser                   string
	ProxyPass                   string
	InitContainerDir            string
	MetricsSyncInterval         int // Frequency of metrics pushes to the controller, sec
	SnapshotSyncInterval        int // Frequency of snapshot pushes to events api, sec
	AgentServerPort             int32
	NsToMonitor                 []string
	NsToMonitorExclude          []string
	DeploysToDashboard          []string
	NodesToMonitor              []string
	NodesToMonitorExclude       []string
	NsToInstrument              []string
	NsToInstrumentExclude       []string
	NSInstrumentRule            []AgentRequest
	InstrumentationMethod       InstrumentationMethod
	DefaultInstrumentationTech  TechnologyName
	BiqService                  string
	InstrumentContainer         string //all, first, name
	InstrumentMatchString       []string
	InitRequestMem              string
	InitRequestCpu              string
	BiqRequestMem               string
	BiqRequestCpu               string
	LogLines                    int //0 - no logging
	PodEventNumber              int
	SecretVersion               string
	SchemaUpdateCache           []string
	LogLevel                    string
	OverconsumptionThreshold    int
	InstrumentationUpdated      bool
}

func IsBreakingProperty(fieldName string) bool {
	arr := []string{"AgentNamespace", "AppName", "TierName", "NodeName", "AppID", "TierID", "NodeID", "Account", "GlobalAccount", "AccessKey", "ControllerUrl",
		"ControllerPort", "RestAPIUrl", "SSLEnabled", "SystemSSLCert", "AgentSSLCert", "EventKey", "EventServiceUrl", "RestAPICred"}
	for _, s := range arr {
		if s == fieldName {
			return false
		}
	}
	return true
}

func GetDefaultProperties() *AppDBag {
	bag := AppDBag{
		AppName:                     "K8s-Cluster-Agent",
		TierName:                    "ClusterAgent",
		NodeName:                    "Node1",
		AgentServerPort:             8989,
		SystemSSLCert:               "/opt/appdynamics/ssl/systemSSL.crt",
		AgentSSLCert:                "",
		EventAPILimit:               100,
		MetricsSyncInterval:         60,
		SnapshotSyncInterval:        30,
		PodSchemaName:               "kube_pod_snapshots",
		NodeSchemaName:              "kube_node_snapshots",
		EventSchemaName:             "kube_event_snapshots",
		ContainerSchemaName:         "kube_container_snapshots",
		JobSchemaName:               "kube_jobs",
		LogSchemaName:               "kube_logs",
		EpSchemaName:                "kube_endpoints",
		NsSchemaName:                "kube_ns_snapshots",
		RqSchemaName:                "kube_rq_snapshots",
		DeploySchemaName:            "kube_deploy_snapshots",
		RSSchemaName:                "kube_rs_snapshots",
		DaemonSchemaName:            "kube_daemon_snapshots",
		DashboardTemplatePath:       "/opt/appdynamics/templates/cluster-template.json",
		DashboardSuffix:             "SUMMARY",
		DashboardDelayMin:           2,
		DeploysToDashboard:          []string{},
		InstrumentationMethod:       "none",
		DefaultInstrumentationTech:  "java",
		BiqService:                  "none",
		InstrumentContainer:         "first",
		InstrumentMatchString:       []string{},
		InitContainerDir:            "/opt/temp",
		AgentLabel:                  "appd-agent",
		AgentLogOverride:            "",
		AgentUserOverride:           "",
		AgentEnvVar:                 "JAVA_OPTS",
		AppNameLiteral:              "",
		AppDAppLabel:                "appd-app",
		AppDTierLabel:               "appd-tier",
		AppDAnalyticsLabel:          "appd-biq",
		AgentMountName:              "appd-agent-repo",
		AgentMountPath:              "/opt/appdynamics",
		AppLogMountName:             "appd-volume",
		AppLogMountPath:             "/opt/appdlogs",
		JDKMountName:                "jdk-repo",
		JDKMountPath:                "$JAVA_HOME/lib",
		AnalyticsAgentUrl:           "http://analytics-proxy:9090",
		AnalyticsAgentContainerName: "appd-analytics-agent",
		AppDInitContainerName:       "appd-agent-attach",
		AnalyticsAgentImage:         "docker.io/appdynamics/analytics-agent:latest",
		AppDJavaAttachImage:         "docker.io/appdynamics/java-agent:latest",
		AppDDotNetAttachImage:       "docker.io/appdynamics/dotnet-core-agent:latest",
		NsToMonitor:                 []string{},
		NsToMonitorExclude:          []string{},
		NodesToMonitor:              []string{},
		NodesToMonitorExclude:       []string{},
		NsToInstrument:              []string{},
		NsToInstrumentExclude:       []string{},
		NSInstrumentRule:            []AgentRequest{},
		InitRequestMem:              "50",
		InitRequestCpu:              "0.1",
		BiqRequestMem:               "600",
		BiqRequestCpu:               "0.1",
		ProxyUrl:                    "",
		ProxyUser:                   "",
		ProxyPass:                   "",
		LogLines:                    0, //0 - no logging}
		PodEventNumber:              1,
		LogLevel:                    "info",
		OverconsumptionThreshold:    80,
		InstrumentationUpdated:      false,
	}

	return &bag
}
