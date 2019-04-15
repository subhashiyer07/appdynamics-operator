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
	Namespaces    []string              `json: "namespaces"`
	AppDAppLabel  string                `json: "appDAppLabel"`
	AppDTierLabel string                `json: "appDTierLabel"`
	Tech          TechnologyName        `json: "tech"`
	ContainerName string                `json: "containerName"`
	Version       string                `json: "version"`
	MatchString   []string              `json: "MatchString"` //string matched against deployment names and labels, supports regex
	Method        InstrumentationMethod `json: "instrumentationMethod"`
	BiQ           string                `json: "biQ"` //"sidecar" or reference to the remote analytics agent
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
	AgentServerPort             int
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
		RestAPICred:                 "k8sresty@customer1:k8sresty",
		ControllerUrl:               "http://455controllernossh-k8sbiqtest-eq2w7bwd.srv.ravcloud.com:8090",
		Account:                     "",
		GlobalAccount:               "",
		EventServiceUrl:             "http://455controllernossh-k8sbiqtest-eq2w7bwd.srv.ravcloud.com:9080",
		EventKey:                    "5be04dca-5c04-491e-b055-bc580ba9d7aa",
		AppName:                     "K8s-Cluster-Agent",
		TierName:                    "ClusterAgent",
		NodeName:                    "Node1",
		AgentServerPort:             8989,
		SystemSSLCert:               "/opt/appd/ssl/system.crt",
		AgentSSLCert:                "/opt/appd/ssl/agent.crt",
		EventAPILimit:               100,
		MetricsSyncInterval:         60,
		SnapshotSyncInterval:        15,
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
		DashboardTemplatePath:       "/usr/local/go/src/github.com/sjeltuhin/clusterAgent/templates/k8s_dashboard_template.json",
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
		AgentEnvVar:                 "JAVA_OPTS",
		AppDAppLabel:                "appd-app",
		AppDTierLabel:               "appd-tier",
		AppDAnalyticsLabel:          "appd-biq",
		AgentMountName:              "appd-agent-repo",
		AgentMountPath:              "/opt/appdynamics",
		AppLogMountName:             "appd-volume",
		AppLogMountPath:             "/opt/appdlogs",
		JDKMountName:                "jdk-repo",
		JDKMountPath:                "$JAVA_HOME/lib",
		NodeNamePrefix:              "",
		AnalyticsAgentUrl:           "http://analytics-proxy:9090",
		AnalyticsAgentContainerName: "appd-analytics-agent",
		AppDInitContainerName:       "appd-agent-attach",
		AnalyticsAgentImage:         "sashaz/analytics-agent@sha256:ff776bdf3beed9f4bdf638d16b5a688d9e1c0fc124ce1282bef1851c122397e4",
		AppDJavaAttachImage:         "sashaz/java-agent-attach@sha256:b93f2018b091f4abfd2533e6c194c9e6ecf00fcae861c732f1b771dad1b26a80",
		AppDDotNetAttachImage:       "sashaz/dotnet-agent-attach@sha256:3f5d921eadfa227ffe072caa41e01c3c1fc882c5617ad45d808ffedaa20593a6",
		AppDNodeJSAttachImage:       "latest",
		NsToMonitor:                 []string{},
		NsToMonitorExclude:          []string{},
		NodesToMonitor:              []string{},
		NodesToMonitorExclude:       []string{},
		NsToInstrument:              []string{},
		NsToInstrumentExclude:       []string{},
		NSInstrumentRule:            []AgentRequest{},
		InitRequestMem:              "50",
		InitRequestCpu:              "0.1",
		BiqRequestMem:               "200",
		BiqRequestCpu:               "0.1",
		ProxyUrl:                    "",
		ProxyUser:                   "",
		ProxyPass:                   "",
		LogLines:                    0, //0 - no logging}
		PodEventNumber:              3,
		LogLevel:                    "info",
		OverconsumptionThreshold:    80,
	}

	return &bag
}
