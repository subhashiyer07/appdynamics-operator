package clustercollector

import (
	"context"
	"fmt"
	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

func setClusterCollectorConfigDefaults(clusterCollector *appdynamicsv1alpha1.Clustercollector) {

	if clusterCollector.Spec.Image == "" {
		clusterCollector.Spec.Image = "vikyath/infra-agent-cluster-collector:latest"
	}
	if clusterCollector.Spec.ServiceAccountName == "" {
		clusterCollector.Spec.ServiceAccountName = "appdynamics-operator"
	}
	if clusterCollector.Spec.LogLevel == "" {
		clusterCollector.Spec.LogLevel = "INFO"
	}
	if clusterCollector.Spec.ExporterAddress == "" {
		clusterCollector.Spec.ExporterAddress = "127.0.0.1"
	}
	if clusterCollector.Spec.ExporterPort == 0 {
		clusterCollector.Spec.ExporterPort = 9100
	}
}

func setInfraAgentConfigsDefaults(clusterCollector *appdynamicsv1alpha1.Clustercollector) {
	if clusterCollector.Spec.SystemConfigs.CollectorLibSocketUrl == ""{
		clusterCollector.Spec.SystemConfigs.CollectorLibSocketUrl = "tcp://127.0.0.1"
	}
	if clusterCollector.Spec.SystemConfigs.CollectorLibPort == "" {
		clusterCollector.Spec.SystemConfigs.CollectorLibPort = "42387"
	}
	if clusterCollector.Spec.SystemConfigs.ConfigChangeScanPeriod == 0 {
		clusterCollector.Spec.SystemConfigs.ConfigChangeScanPeriod = 5 //sec
	}
	if clusterCollector.Spec.SystemConfigs.HttpBasicAuthEnabled == false {
		clusterCollector.Spec.SystemConfigs.HttpBasicAuthEnabled = true
	}
	if clusterCollector.Spec.SystemConfigs.ConfigStaleGracePeriod == 0 {
		clusterCollector.Spec.SystemConfigs.ConfigStaleGracePeriod = 600  // sec
	}
	if clusterCollector.Spec.SystemConfigs.HttpClientTimeOut == 0 {
		clusterCollector.Spec.SystemConfigs.HttpClientTimeOut = 10000 // ms
	}
	if clusterCollector.Spec.SystemConfigs.ClientLibSendUrl == "" {
		clusterCollector.Spec.SystemConfigs.ClientLibSendUrl = "tcp://127.0.0.1:42387"
	}
	if clusterCollector.Spec.SystemConfigs.ClientLibRecvUrl == "" {
		clusterCollector.Spec.SystemConfigs.ClientLibRecvUrl = "tcp://127.0.0.1:4238"
	}
	if clusterCollector.Spec.SystemConfigs.LogLevel == "" {
		clusterCollector.Spec.SystemConfigs.LogLevel = "INFO"
	}
	if clusterCollector.Spec.SystemConfigs.DebugPort == "" {
		clusterCollector.Spec.SystemConfigs.DebugPort = "39987"
	}
}

func setHostCollectorConfigDefaults(clusterCollector *appdynamicsv1alpha1.Clustercollector) {
	if clusterCollector.Spec.HostCollector.Image == "" {
		clusterCollector.Spec.HostCollector.Image = "vikyath/host-collector:latest"
	}

	if clusterCollector.Spec.HostCollector.ServiceAccountName == "" {
		clusterCollector.Spec.HostCollector.ServiceAccountName = "appdynamics-operator"
	}
	if clusterCollector.Spec.HostCollector.Name == "" {
		clusterCollector.Spec.HostCollector.Name = HOST_COLLECTOR_NAME
	}
}

func validateControllerUrl(controllerUrl string) (error, string, uint16, string) {
	if strings.Contains(controllerUrl, "http") {
		arr := strings.Split(controllerUrl, ":")
		if len(arr) > 3 || len(arr) < 2 {
			return fmt.Errorf("Controller Url is invalid. Use this format: protocol://url:port"), "", 0, ""
		}
		protocol := arr[0]
		controllerDns := strings.TrimLeft(arr[1], "//")
		controllerPort := 0
		if len(arr) != 3 {
			if strings.Contains(protocol, "s") {
				controllerPort = 443
			} else {
				controllerPort = 80
			}
		} else {
			port, errPort := strconv.Atoi(arr[2])
			if errPort != nil {
				return fmt.Errorf("Controller port is invalid. %v", errPort), "", 0, ""
			}
			controllerPort = port
		}

		ssl := "false"
		if strings.Contains(protocol, "s") {
			ssl = "true"
		}
		return nil, controllerDns, uint16(controllerPort), ssl
	} else {
		return fmt.Errorf("Controller Url is invalid. Use this format: protocol://dns:port"), "", 0, ""
	}
}

func hasBreakingChanges(clusterCollector *appdynamicsv1alpha1.Clustercollector, existingDeployment *appsv1.Deployment) (bool, bool) {
	fmt.Println("Checking for breaking changes...")
	if clusterCollector.Spec.Image != "" && existingDeployment.Spec.Template.Spec.Containers[0].Image != clusterCollector.Spec.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDeployment.Spec.Template.Spec.Containers[0].Image, clusterCollector.Spec.Image)
		existingDeployment.Spec.Template.Spec.Containers[0].Image = clusterCollector.Spec.Image
		return false, true
	}
	return false, false
}

func updateStatus(clusterCollector *appdynamicsv1alpha1.Clustercollector, client client.Client) error {
	clusterCollector.Status.LastUpdateTime = metav1.Now()

	if errInstance := client.Update(context.TODO(), clusterCollector); errInstance != nil {
		return fmt.Errorf("Unable to update clustercollector instance. %v", errInstance)
	}
	log.Info("Clustercollector instance updated successfully", "clusterCollector.Namespace", clusterCollector.Namespace, "Date", clusterCollector.Status.LastUpdateTime)

	err := client.Status().Update(context.TODO(), clusterCollector)
	if err != nil {
		log.Error(err, "Failed to update cluster collector status", "clusterCollector.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
	} else {
		log.Info("Clustercollector status updated successfully", "clusterCollector.Namespace", clusterCollector.Namespace, "Date", clusterCollector.Status.LastUpdateTime)
	}
	return err
}

func createConfigMap(client client.Client, cm *corev1.ConfigMap) error {
    existingConfigMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, existingConfigMap)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load %s. %v", cm.Name, err)
	}

	if create {
		e := client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create %s. %v", cm.Name, e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update %s. %v", cm.Name, e)
		}
		fmt.Println("Infra Agent Configmap updated")
	}
	return nil
}