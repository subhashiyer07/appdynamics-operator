package clusteragent

import (
	"reflect"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func slicesEqual(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	// create a map of string -> int
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		// 0 value for int is 0, so just increment a counter for the string
		diff[_x]++
	}
	for _, _y := range y {
		// If the string _y is not in diff bail out early
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y] -= 1
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	if len(diff) == 0 {
		return true
	}
	return false
}

func reconcileBag(bag *appdynamicsv1alpha1.AppDBag, clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret) {
	bag.InstrumentationUpdated = false
	bag.SecretVersion = secret.ResourceVersion
	bag.AgentNamespace = clusterAgent.Namespace
	bag.ControllerUrl = clusterAgent.Spec.ControllerUrl

	if clusterAgent.Spec.Account != "" {
		bag.Account = clusterAgent.Spec.Account
	}
	if clusterAgent.Spec.GlobalAccount != "" {
		bag.GlobalAccount = clusterAgent.Spec.GlobalAccount
	}

	if clusterAgent.Spec.EventServiceUrl != "" {
		bag.EventServiceUrl = clusterAgent.Spec.EventServiceUrl
	}

	if clusterAgent.Spec.AppName != "" {
		bag.AppName = clusterAgent.Spec.AppName
	}

	if clusterAgent.Spec.SystemSSLCert != "" {
		bag.SystemSSLCert = clusterAgent.Spec.SystemSSLCert
	}

	if clusterAgent.Spec.AgentSSLCert != "" {
		bag.AgentSSLCert = clusterAgent.Spec.AgentSSLCert
	}

	if clusterAgent.Spec.ProxyUrl != "" {
		bag.ProxyUrl = clusterAgent.Spec.ProxyUrl
	}

	if clusterAgent.Spec.ProxyUser != "" {
		bag.ProxyUser = clusterAgent.Spec.ProxyUser
	}

	if clusterAgent.Spec.ProxyPass != "" {
		bag.ProxyPass = clusterAgent.Spec.ProxyPass
	}

	if clusterAgent.Spec.AgentServerPort > 0 {
		bag.AgentServerPort = clusterAgent.Spec.AgentServerPort
	}

	if clusterAgent.Spec.EventAPILimit > 0 {
		bag.EventAPILimit = clusterAgent.Spec.EventAPILimit
	}

	if clusterAgent.Spec.MetricsSyncInterval > 0 {
		bag.MetricsSyncInterval = clusterAgent.Spec.MetricsSyncInterval
	}

	if clusterAgent.Spec.SnapshotSyncInterval > 0 {
		bag.SnapshotSyncInterval = clusterAgent.Spec.SnapshotSyncInterval
	}

	if clusterAgent.Spec.LogLines > 0 {
		bag.LogLines = clusterAgent.Spec.LogLines
	}

	if clusterAgent.Spec.LogLevel != "" {
		bag.LogLevel = clusterAgent.Spec.LogLevel
	}

	if clusterAgent.Spec.PodEventNumber > 0 {
		bag.PodEventNumber = clusterAgent.Spec.PodEventNumber
	}

	if clusterAgent.Spec.OverconsumptionThreshold > 0 {
		bag.OverconsumptionThreshold = clusterAgent.Spec.OverconsumptionThreshold
	}

	////////

	if clusterAgent.Spec.PodSchemaName != "" {
		bag.PodSchemaName = clusterAgent.Spec.PodSchemaName
	}

	if clusterAgent.Spec.NodeSchemaName != "" {
		bag.NodeSchemaName = clusterAgent.Spec.NodeSchemaName
	}

	if clusterAgent.Spec.EventSchemaName != "" {
		bag.EventSchemaName = clusterAgent.Spec.EventSchemaName
	}

	if clusterAgent.Spec.ContainerSchemaName != "" {
		bag.ContainerSchemaName = clusterAgent.Spec.ContainerSchemaName
	}

	if clusterAgent.Spec.JobSchemaName != "" {
		bag.JobSchemaName = clusterAgent.Spec.JobSchemaName
	}

	if clusterAgent.Spec.LogSchemaName != "" {
		bag.LogSchemaName = clusterAgent.Spec.LogSchemaName
	}

	if clusterAgent.Spec.EpSchemaName != "" {
		bag.EpSchemaName = clusterAgent.Spec.EpSchemaName
	}

	if clusterAgent.Spec.NsSchemaName != "" {
		bag.NsSchemaName = clusterAgent.Spec.NsSchemaName
	}

	if clusterAgent.Spec.RqSchemaName != "" {
		bag.RqSchemaName = clusterAgent.Spec.RqSchemaName
	}

	if clusterAgent.Spec.DeploySchemaName != "" {
		bag.DeploySchemaName = clusterAgent.Spec.DeploySchemaName
	}

	if clusterAgent.Spec.RSSchemaName != "" {
		bag.RSSchemaName = clusterAgent.Spec.RSSchemaName
	}

	if clusterAgent.Spec.DaemonSchemaName != "" {
		bag.DaemonSchemaName = clusterAgent.Spec.DaemonSchemaName
	}
	///

	if clusterAgent.Spec.DashboardSuffix != "" {
		bag.DashboardSuffix = clusterAgent.Spec.DashboardSuffix
	}

	if clusterAgent.Spec.DashboardDelayMin > 0 {
		bag.DashboardDelayMin = clusterAgent.Spec.DashboardDelayMin
	}

	if clusterAgent.Spec.DashboardTemplatePath != "" {
		bag.DashboardTemplatePath = clusterAgent.Spec.DashboardTemplatePath
	}

	if bag.NetVizPort != clusterAgent.Spec.NetVizPort {
		bag.NetVizPort = clusterAgent.Spec.NetVizPort
	}

	if bag.UniqueHostID != clusterAgent.Spec.UniqueHostID {
		bag.UniqueHostID = clusterAgent.Spec.UniqueHostID
	}

	if bag.InstrumentationMethod != appdynamicsv1alpha1.InstrumentationMethod(clusterAgent.Spec.InstrumentationMethod) {
		bag.InstrumentationUpdated = true
		bag.InstrumentationMethod = appdynamicsv1alpha1.InstrumentationMethod(clusterAgent.Spec.InstrumentationMethod)
	}

	if clusterAgent.Spec.DefaultInstrumentationTech != "" {
		bag.DefaultInstrumentationTech = appdynamicsv1alpha1.TechnologyName(clusterAgent.Spec.DefaultInstrumentationTech)
	}

	if reflect.DeepEqual(bag.InstrumentMatchString, clusterAgent.Spec.InstrumentMatchString) == false {
		bag.InstrumentMatchString = make([]string, len(clusterAgent.Spec.InstrumentMatchString))
		copy(bag.InstrumentMatchString, clusterAgent.Spec.InstrumentMatchString)
		bag.InstrumentationUpdated = true
	}

	if reflect.DeepEqual(bag.NsToInstrument, clusterAgent.Spec.NsToInstrument) == false {
		bag.NsToInstrument = make([]string, len(clusterAgent.Spec.NsToInstrument))
		copy(bag.NsToInstrument, clusterAgent.Spec.NsToInstrument)
		bag.InstrumentationUpdated = true
	}

	if reflect.DeepEqual(bag.NsToInstrumentExclude, clusterAgent.Spec.NsToInstrumentExclude) == false {
		bag.NsToInstrumentExclude = make([]string, len(clusterAgent.Spec.NsToInstrumentExclude))
		copy(bag.NsToInstrumentExclude, clusterAgent.Spec.NsToInstrumentExclude)
		bag.InstrumentationUpdated = true
	}

	if len(clusterAgent.Spec.NsToMonitor) > 0 {
		bag.NsToMonitor = make([]string, len(clusterAgent.Spec.NsToMonitor))
		copy(bag.NsToMonitor, clusterAgent.Spec.NsToMonitor)
	}

	if len(clusterAgent.Spec.NsToMonitorExclude) > 0 {
		bag.NsToMonitorExclude = make([]string, len(clusterAgent.Spec.NsToMonitorExclude))
		copy(bag.NsToMonitorExclude, clusterAgent.Spec.NsToMonitorExclude)
	}

	if len(clusterAgent.Spec.NodesToMonitor) > 0 {
		bag.NodesToMonitor = make([]string, len(clusterAgent.Spec.NodesToMonitor))
		copy(bag.NodesToMonitor, clusterAgent.Spec.NodesToMonitor)
	}

	if len(clusterAgent.Spec.NodesToMonitorExclude) > 0 {
		bag.NodesToMonitorExclude = make([]string, len(clusterAgent.Spec.NodesToMonitorExclude))
		copy(bag.NodesToMonitorExclude, clusterAgent.Spec.NodesToMonitorExclude)
	}

	if reflect.DeepEqual(bag.NSInstrumentRule, clusterAgent.Spec.InstrumentRule) == false {
		bag.NSInstrumentRule = make([]appdynamicsv1alpha1.AgentRequest, len(clusterAgent.Spec.InstrumentRule))
		copy(bag.NSInstrumentRule, clusterAgent.Spec.InstrumentRule)
		bag.InstrumentationUpdated = true
	}

	if bag.AgentLogOverride != clusterAgent.Spec.AgentLogOverride {
		bag.AgentLogOverride = clusterAgent.Spec.AgentLogOverride
	}

	if bag.AgentUserOverride != clusterAgent.Spec.AgentUserOverride {
		bag.AgentUserOverride = clusterAgent.Spec.AgentUserOverride
	}

	if clusterAgent.Spec.AnalyticsAgentImage != "" {
		bag.AnalyticsAgentImage = clusterAgent.Spec.AnalyticsAgentImage
	}

	if clusterAgent.Spec.AppDJavaAttachImage != "" {
		bag.AppDJavaAttachImage = clusterAgent.Spec.AppDJavaAttachImage
	}

	if clusterAgent.Spec.AppDDotNetAttachImage != "" {
		bag.AppDDotNetAttachImage = clusterAgent.Spec.AppDDotNetAttachImage
	}

	if clusterAgent.Spec.BiqService != bag.BiqService {
		bag.BiqService = clusterAgent.Spec.BiqService
		bag.InstrumentationUpdated = true
	}

	if clusterAgent.Spec.InstrumentContainer != "" {
		bag.InstrumentContainer = clusterAgent.Spec.InstrumentContainer
	}

	if clusterAgent.Spec.InitContainerDir != "" {
		bag.InitContainerDir = clusterAgent.Spec.InitContainerDir
	}

	if clusterAgent.Spec.AgentLabel != "" {
		bag.AgentLabel = clusterAgent.Spec.AgentLabel
	}

	if clusterAgent.Spec.AgentEnvVar != "" {
		bag.AgentEnvVar = clusterAgent.Spec.AgentEnvVar
		bag.InstrumentationUpdated = true
	}

	if bag.AppNameLiteral != clusterAgent.Spec.AppNameLiteral {
		bag.AppNameLiteral = clusterAgent.Spec.AppNameLiteral
		bag.InstrumentationUpdated = true
	}

	if clusterAgent.Spec.AppDAppLabel != bag.AppDAppLabel {
		bag.AppDAppLabel = clusterAgent.Spec.AppDAppLabel
		bag.InstrumentationUpdated = true
	}

	if clusterAgent.Spec.AppDTierLabel != bag.AppDTierLabel {
		bag.AppDTierLabel = clusterAgent.Spec.AppDTierLabel
		bag.InstrumentationUpdated = true
	}

	if clusterAgent.Spec.AppDAnalyticsLabel != "" {
		bag.AppDAnalyticsLabel = clusterAgent.Spec.AppDAnalyticsLabel
	}

	if clusterAgent.Spec.AgentMountName != "" {
		bag.AgentMountName = clusterAgent.Spec.AgentMountName
	}

	if clusterAgent.Spec.AgentMountPath != "" {
		bag.AgentMountPath = clusterAgent.Spec.AgentMountPath
	}

	if clusterAgent.Spec.AppLogMountName != "" {
		bag.AppLogMountName = clusterAgent.Spec.AppLogMountName
	}

	if clusterAgent.Spec.AppLogMountPath != "" {
		bag.AppLogMountPath = clusterAgent.Spec.AppLogMountPath
	}

	if clusterAgent.Spec.JDKMountName != "" {
		bag.JDKMountName = clusterAgent.Spec.JDKMountName
	}

	if clusterAgent.Spec.JDKMountPath != "" {
		bag.JDKMountPath = clusterAgent.Spec.JDKMountPath
	}

	if clusterAgent.Spec.NodeNamePrefix != "" {
		bag.NodeNamePrefix = clusterAgent.Spec.NodeNamePrefix
	}

	if clusterAgent.Spec.AnalyticsAgentUrl != "" {
		bag.AnalyticsAgentUrl = clusterAgent.Spec.AnalyticsAgentUrl
	}

	if clusterAgent.Spec.AnalyticsAgentContainerName != "" {
		bag.AnalyticsAgentContainerName = clusterAgent.Spec.AnalyticsAgentContainerName
	}

	if clusterAgent.Spec.AppDInitContainerName != "" {
		bag.AppDInitContainerName = clusterAgent.Spec.AppDInitContainerName
	}

	if clusterAgent.Spec.InitRequestMem != "" {
		bag.InitRequestMem = clusterAgent.Spec.InitRequestMem
	}

	if clusterAgent.Spec.InitRequestCpu != "" {
		bag.InitRequestCpu = clusterAgent.Spec.InitRequestCpu
	}

	if clusterAgent.Spec.InitRequestMem != "" {
		bag.InitRequestMem = clusterAgent.Spec.InitRequestMem
	}

	if clusterAgent.Spec.BiqRequestMem != "" {
		bag.BiqRequestCpu = clusterAgent.Spec.BiqRequestCpu
	}

}
