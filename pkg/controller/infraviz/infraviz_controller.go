package infraviz

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	"github.com/Appdynamics/appdynamics-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_infraviz")

const (
	AGENT_SECRET_NAME        string = "cluster-agent-secret"
	AGENT_CONFIG_NAME        string = "ma-config"
	AGENT_LOG_CONFIG_NAME    string = "ma-log-config"
	AGENT_NETVIZ_CONFIG_NAME string = "netviz-config"
	BIQPORT                  int32  = 9090
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new InfraViz Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileInfraViz{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("infraviz-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource InfraViz
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.InfraViz{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner InfraViz
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.InfraViz{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileInfraViz{}

// ReconcileInfraViz reconciles a InfraViz object
type ReconcileInfraViz struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a InfraViz object and makes changes based on the state read
// and what is in the InfraViz.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileInfraViz) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling InfraViz")

	// Fetch the InfraViz instance
	infraViz := &appdynamicsv1alpha1.InfraViz{}
	err := r.client.Get(context.TODO(), request.NamespacedName, infraViz)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	r.scheme.Default(infraViz)

	desiredDS := r.newInfraVizDaemonSet(infraViz)

	// Set InfraViz instance as the owner and controller
	if err := controllerutil.SetControllerReference(infraViz, desiredDS, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if the Daemonset already exists
	existingDs := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: infraViz.Name, Namespace: infraViz.Namespace}, existingDs)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Daemon Set", "Namespace", infraViz.Namespace, "Name", infraViz.Name)
		err = r.client.Create(context.TODO(), desiredDS)
		if err != nil {
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	//if any breaking changes, restart ds
	hasBreakingChanges, errConf := r.ensureConfigMap(infraViz)
	if errConf != nil {
		return reconcile.Result{}, errConf
	}

	if hasBreakingChanges {
		err := r.restartDaemonSet(infraViz)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if hasDSpecChanged(&existingDs.Spec, &desiredDS.Spec, &infraViz.Spec) {
		err = r.client.Update(context.TODO(), desiredDS)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	r.updateStatus(infraViz)

	return reconcile.Result{}, nil
}

func (r *ReconcileInfraViz) updateStatus(infraViz *appdynamicsv1alpha1.InfraViz) error {
	infraViz.Status.LastUpdateTime = metav1.Now()
	infraViz.Status.Version = version.Version

	podList, err := r.getInfraVizPods(infraViz)
	if err != nil {
		return fmt.Errorf("Unable to update InfraViz status. %v", err)
	}

	infraViz.Status.Nodes = make(map[string]string)
	for _, pod := range podList.Items {
		name := pod.Name
		status := pod.Status.Phase
		infraViz.Status.Nodes[name] = string(status)
	}

	updatedStatus := infraViz.Status

	infraViz.Status = appdynamicsv1alpha1.InfraVizStatus{}

	if errInstance := r.client.Update(context.TODO(), infraViz); errInstance != nil {
		return fmt.Errorf("Unable to update clusteragent instance. %v", errInstance)
	}
	log.Info("ClusterAgent instance updated successfully", "clusterAgent.Namespace", infraViz.Namespace, "Date", infraViz.Status.LastUpdateTime)

	infraViz.Status = updatedStatus
	err = r.client.Status().Update(context.TODO(), infraViz)
	if err != nil {
		log.Error(err, "Failed to update cluster agent status", "clusterAgent.Namespace", infraViz.Namespace, "Deployment.Name", infraViz.Name)
	} else {
		log.Info("ClusterAgent status updated successfully", "clusterAgent.Namespace", infraViz.Namespace, "Date", infraViz.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileInfraViz) ensureConfigMap(infraViz *appdynamicsv1alpha1.InfraViz) (bool, error) {
	breakingChanges := false

	logLevel := "info"

	errVal, controllerDns, port, sslEnabled := validateControllerUrl(infraViz.Spec.ControllerUrl)
	if errVal != nil {
		return breakingChanges, errVal
	}
	fmt.Printf("port=%d\n", port)

	eventUrl := infraViz.Spec.EventServiceUrl
	if eventUrl == "" {
		if strings.Contains(controllerDns, "appdynamics.com") {
			//saas
			eventUrl = "https://analytics.api.appdynamics.com"
		} else {
			protocol := "http"
			if sslEnabled == "true" {
				protocol = "https"
			}
			eventUrl = fmt.Sprintf("%s://%s:9080", protocol, controllerDns)
		}
	}
	var proxyHost, proxyPort, proxyUser, proxyPass string
	if infraViz.Spec.ProxyUrl != "" {
		arr := strings.Split(infraViz.Spec.ProxyUrl, ":")
		if len(arr) != 3 {
			fmt.Println("ProxyUrl is invalid. Use this format: protocol://domain:port")
		}
		proxyHost = strings.TrimLeft(arr[1], "//")
		proxyPort = arr[2]
	}

	if infraViz.Spec.ProxyUser != "" {
		arr := strings.Split(infraViz.Spec.ProxyUser, "@")
		if len(arr) != 2 {
			fmt.Println("ProxyUser is invalid. Use this format: user@pass")
		}
		proxyUser = arr[0]
		proxyPass = arr[1]
	}

	if infraViz.Spec.LogLevel != "" {
		logLevel = infraViz.Spec.LogLevel
	}

	if infraViz.Spec.NetVizPort > 0 {
		r.ensureNetVizConfig(infraViz)
	}

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "ma-config", Namespace: infraViz.Namespace}, cm)

	create := false
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Config map not found. Creating...\n")
		//configMap does not exist. Create
		cm.Name = "ma-config"
		cm.Namespace = infraViz.Namespace
		cm.Data = make(map[string]string)
		create = true

	} else if err != nil {
		return breakingChanges, fmt.Errorf("Failed to load configMap ma-config. %v", err)
	}

	if !create {
		if cm.Data["APPDYNAMICS_LOG_LEVEL"] != logLevel ||
			cm.Data["APPDYNAMICS_LOG_STDOUT"] != strconv.FormatBool(infraViz.Spec.StdoutLogging) {
			breakingChanges = true
			e := r.ensureLogConfig(infraViz, logLevel)
			if e != nil {
				return breakingChanges, e
			}
		}

		if cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] != infraViz.Spec.Account ||
			cm.Data["APPDYNAMICS_AGENT_GLOBAL_ACCOUNT_NAME"] != infraViz.Spec.GlobalAccount ||
			cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] != controllerDns ||
			cm.Data["APPDYNAMICS_CONTROLLER_PORT"] != strconv.Itoa(int(port)) ||
			cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] != sslEnabled ||
			cm.Data["EVENT_ENDPOINT"] != eventUrl ||
			cm.Data["APPDYNAMICS_AGENT_PROXY_HOST"] != proxyHost ||
			cm.Data["APPDYNAMICS_AGENT_PROXY_PORT"] != proxyPort ||
			cm.Data["APPDYNAMICS_AGENT_PROXY_USER"] != proxyUser ||
			cm.Data["APPDYNAMICS_AGENT_PROXY_PASS"] != proxyPass ||
			cm.Data["APPDYNAMICS_AGENT_ENABLE_CONTAINERIDASHOSTID"] != infraViz.Spec.EnableContainerHostId ||
			cm.Data["APPDYNAMICS_SIM_ENABLED"] != infraViz.Spec.EnableServerViz ||
			cm.Data["APPDYNAMICS_DOCKER_ENABLED"] != infraViz.Spec.EnableDockerViz ||
			cm.Data["APPDYNAMICS_AGENT_METRIC_LIMIT"] != infraViz.Spec.MetricsLimit ||
			cm.Data["APPDYNAMICS_MA_PROPERTIES"] != infraViz.Spec.PropertyBag {
			breakingChanges = true
		}

	} else {
		e := r.ensureLogConfig(infraViz, logLevel)
		if e != nil {
			return breakingChanges, e
		}
	}

	cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] = infraViz.Spec.Account
	cm.Data["APPDYNAMICS_AGENT_GLOBAL_ACCOUNT_NAME"] = infraViz.Spec.GlobalAccount
	cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] = controllerDns
	cm.Data["APPDYNAMICS_CONTROLLER_PORT"] = strconv.Itoa(int(port))
	cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] = string(sslEnabled)

	cm.Data["APPDYNAMICS_NETVIZ_AGENT_PORT"] = strconv.Itoa(int(infraViz.Spec.NetVizPort))

	if infraViz.Spec.EnableContainerHostId == "" {
		infraViz.Spec.EnableContainerHostId = "true"
	}
	cm.Data["APPDYNAMICS_AGENT_ENABLE_CONTAINERIDASHOSTID"] = infraViz.Spec.EnableContainerHostId

	if infraViz.Spec.EnableServerViz == "" {
		infraViz.Spec.EnableServerViz = "true"
	}

	cm.Data["APPDYNAMICS_SIM_ENABLED"] = infraViz.Spec.EnableServerViz

	if infraViz.Spec.EnableDockerViz == "" {
		infraViz.Spec.EnableDockerViz = "true"
	}

	cm.Data["APPDYNAMICS_DOCKER_ENABLED"] = infraViz.Spec.EnableDockerViz

	cm.Data["EVENT_ENDPOINT"] = eventUrl
	cm.Data["APPDYNAMICS_AGENT_PROXY_HOST"] = proxyHost
	cm.Data["APPDYNAMICS_AGENT_PROXY_PORT"] = proxyPort
	cm.Data["APPDYNAMICS_AGENT_PROXY_USER"] = proxyUser
	cm.Data["APPDYNAMICS_AGENT_PROXY_PASS"] = proxyPass
	cm.Data["APPDYNAMICS_AGENT_METRIC_LIMIT"] = infraViz.Spec.MetricsLimit
	cm.Data["APPDYNAMICS_LOG_LEVEL"] = logLevel
	cm.Data["APPDYNAMICS_LOG_STDOUT"] = strconv.FormatBool(infraViz.Spec.StdoutLogging)
	cm.Data["APPDYNAMICS_MA_PROPERTIES"] = infraViz.Spec.PropertyBag

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return breakingChanges, fmt.Errorf("Unable to create MA config map. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return breakingChanges, fmt.Errorf("Unable to update MA config map. %v", e)
		}
	}

	return breakingChanges, nil
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

func (r *ReconcileInfraViz) restartDaemonSet(infraViz *appdynamicsv1alpha1.InfraViz) error {
	podList, err := r.getInfraVizPods(infraViz)

	if err != nil {
		return err
	}

	for _, p := range podList.Items {
		err = r.client.Delete(context.TODO(), &p)
		if err != nil {
			return fmt.Errorf("Unable to delete InfraViz pod. %v", err)
		}
	}

	return nil
}

func (r *ReconcileInfraViz) getInfraVizPods(infraViz *appdynamicsv1alpha1.InfraViz) (*corev1.PodList, error) {
	podList := corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForInfraViz(infraViz))
	filter := &client.ListOptions{
		Namespace:     infraViz.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), filter, &podList)
	if err != nil {
		return nil, fmt.Errorf("Unable to load InfraViz pods. %v", err)
	}

	return &podList, nil
}

func (r *ReconcileInfraViz) newInfraVizDaemonSet(infraViz *appdynamicsv1alpha1.InfraViz) *appsv1.DaemonSet {
	netviz := false
	if infraViz.Spec.NetVizPort > 0 {
		netviz = true
	}

	if infraViz.Spec.BiqPort == 0 {
		infraViz.Spec.BiqPort = BIQPORT
	}

	errSvc := r.ensureAgentService(infraViz)
	if errSvc != nil {
		fmt.Printf("Issues with InfraViz service: %v", errSvc)
	}
	r.ensureSecret(infraViz)

	selector := labelsForInfraViz(infraViz)
	podSpec := r.newPodSpecForCR(infraViz, netviz)

	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      infraViz.Name,
			Namespace: infraViz.Namespace,
			Labels:    selector,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec:       podSpec,
			},
		},
	}

	return &ds
}

func (r *ReconcileInfraViz) newPodSpecForCR(infraViz *appdynamicsv1alpha1.InfraViz, netviz bool) corev1.PodSpec {
	trueVar := true
	if infraViz.Spec.Image == "" {
		infraViz.Spec.Image = "appdynamics/machine-agent-analytics:latest"
	}

	if infraViz.Spec.NetVizImage == "" {
		infraViz.Spec.NetVizImage = "appdynamics/machine-agent-netviz:latest"
	}

	accessKey := corev1.EnvVar{
		Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
				Key:                  "controller-key",
			},
		},
	}

	if infraViz.Spec.Env == nil || len(infraViz.Spec.Env) == 0 {
		infraViz.Spec.Env = []corev1.EnvVar{}
		infraViz.Spec.Env = append(infraViz.Spec.Env, accessKey)
	}

	dir := corev1.HostPathDirectory
	socket := corev1.HostPathSocket

	cm := corev1.EnvFromSource{}
	cm.ConfigMapRef = &corev1.ConfigMapEnvSource{}
	cm.ConfigMapRef.Name = AGENT_CONFIG_NAME

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Args: infraViz.Spec.Args,
			Env:  infraViz.Spec.Env,
			EnvFrom: []corev1.EnvFromSource{
				cm,
			},
			Image:           infraViz.Spec.Image,
			ImagePullPolicy: corev1.PullAlways,
			Name:            "appd-infra-agent",

			Resources: infraViz.Spec.Resources,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &trueVar,
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: infraViz.Spec.BiqPort,
				Protocol:      corev1.ProtocolTCP,
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "hostroot",
				MountPath: "/hostroot",
				ReadOnly:  true,
			}, {
				Name:      "ma-log-volume",
				MountPath: "/opt/appdynamics/conf/logging/log4j.xml",
				SubPath:   "log4j.xml",
				ReadOnly:  true,
			}, {
				Name:      "docker-sock",
				MountPath: "/var/run/docker.sock",
				ReadOnly:  true,
			}},
		}},
		HostNetwork:        true,
		HostPID:            true,
		HostIPC:            true,
		NodeSelector:       infraViz.Spec.NodeSelector,
		ServiceAccountName: "appdynamics-infraviz",
		Tolerations:        infraViz.Spec.Tolerations,
		Volumes: []corev1.Volume{{
			Name: "hostroot",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/", Type: &dir,
				},
			},
		},
			{
				Name: "docker-sock",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/run/docker.sock", Type: &socket,
					},
				},
			}, {
				Name: "ma-log-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: AGENT_LOG_CONFIG_NAME,
						},
					},
				},
			}},
	}

	if netviz {
		resRequest := corev1.ResourceList{}
		resRequest[corev1.ResourceCPU] = resource.MustParse("0.1")
		resRequest[corev1.ResourceMemory] = resource.MustParse("150Mi")

		resLimit := corev1.ResourceList{}
		resLimit[corev1.ResourceCPU] = resource.MustParse("0.2")
		resLimit[corev1.ResourceMemory] = resource.MustParse("300Mi")
		reqs := corev1.ResourceRequirements{Requests: resRequest, Limits: resLimit}

		netVizVolume := corev1.Volume{Name: "netviz-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: AGENT_NETVIZ_CONFIG_NAME,
					},
				},
			}}
		podSpec.Volumes = append(podSpec.Volumes, netVizVolume)

		netVizContainer := corev1.Container{
			Image:           infraViz.Spec.NetVizImage,
			ImagePullPolicy: corev1.PullAlways,
			Name:            "appd-netviz-agent",

			Resources: reqs,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"}},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: infraViz.Spec.NetVizPort,
				Protocol:      corev1.ProtocolTCP,
				HostPort:      infraViz.Spec.NetVizPort,
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "netviz-volume",
				MountPath: "/netviz-agent/conf/agent_config.lua",
				SubPath:   "agent_config.lua",
			}},
		}
		podSpec.Containers = append(podSpec.Containers, netVizContainer)
	}

	return podSpec
}

func hasDSpecChanged(dsSpec *appsv1.DaemonSetSpec, newSpec *appsv1.DaemonSetSpec, ivSpec *appdynamicsv1alpha1.InfraVizSpec) bool {
	if len(dsSpec.Template.Spec.Containers) != len(newSpec.Template.Spec.Containers) {
		return true
	}
	if len(dsSpec.Template.Spec.Containers) == 2 && len(newSpec.Template.Spec.Containers) == 2 &&
		len(dsSpec.Template.Spec.Containers[1].Ports) > 0 && len(newSpec.Template.Spec.Containers[1].Ports) > 0 {
		if dsSpec.Template.Spec.Containers[1].Ports[0].ContainerPort != newSpec.Template.Spec.Containers[1].Ports[0].ContainerPort {
			return true
		}
	}
	currentSpecClone := ivSpec.DeepCopy()
	cloneCurrentSpec(dsSpec, currentSpecClone)
	if !reflect.DeepEqual(ivSpec, currentSpecClone) {
		return true
	}
	return false
}

func cloneCurrentSpec(dsSpec *appsv1.DaemonSetSpec, ivSpec *appdynamicsv1alpha1.InfraVizSpec) {

	ivSpec.Image = ""
	if len(dsSpec.Template.Spec.Containers) >= 1 {
		ivSpec.Image = dsSpec.Template.Spec.Containers[0].Image
	}

	ivSpec.Env = nil
	if len(dsSpec.Template.Spec.Containers) >= 1 && dsSpec.Template.Spec.Containers[0].Env != nil {
		in, out := &dsSpec.Template.Spec.Containers[0].Env, &ivSpec.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	ivSpec.NodeSelector = nil
	if dsSpec.Template.Spec.NodeSelector != nil {
		in, out := &dsSpec.Template.Spec.NodeSelector, &ivSpec.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}

	ivSpec.Tolerations = nil
	if dsSpec.Template.Spec.Tolerations != nil {
		in, out := &dsSpec.Template.Spec.Tolerations, &ivSpec.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	ivSpec.Args = nil
	if len(dsSpec.Template.Spec.Containers) >= 1 && dsSpec.Template.Spec.Containers[0].Args != nil {
		in, out := &dsSpec.Template.Spec.Containers[0].Args, &ivSpec.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}

	ivSpec.Resources = corev1.ResourceRequirements{}
	if len(dsSpec.Template.Spec.Containers) >= 1 {
		dsSpec.Template.Spec.Containers[0].Resources.DeepCopyInto(&ivSpec.Resources)
	}
}

func (r *ReconcileInfraViz) ensureLogConfig(infraViz *appdynamicsv1alpha1.InfraViz, logLevel string) error {
	appender := "FileAppender"
	if infraViz.Spec.StdoutLogging {
		appender = "ConsoleAppender"
	}

	xml := `<?xml version="1.0" encoding="UTF-8" ?>
<!DOCTYPE log4j:configuration SYSTEM "log4j.dtd">
<log4j:configuration xmlns:log4j="http://jakarta.apache.org/log4j/">

    <appender name="ConsoleAppender" class="org.apache.log4j.ConsoleAppender">
        <layout class="org.apache.log4j.PatternLayout">
            <param name="ConversionPattern" value="%d{ABSOLUTE} %5p [%t] %c{1} - %m%n"/>
        </layout>
    </appender>

    <appender name="FileAppender" class="com.singularity.ee.agent.systemagent.SystemAgentLogAppender">
        <param name="File" value="logs/machine-agent.log"/>
        <param name="MaxFileSize" value="5000KB"/>
        <param name="MaxBackupIndex" value="5"/>
        <layout class="org.apache.log4j.PatternLayout">
            <param name="ConversionPattern" value="[%t] %d{DATE} %5p %c{1} - %m%n"/>
        </layout>
    </appender>` + fmt.Sprintf(`
    <logger name="com.singularity" additivity="false">
        <level value="%s"/>
        <appender-ref ref="%s"/>
    </logger>

    <logger name="com.appdynamics" additivity="false">
        <level value="%s"/>
        <appender-ref ref="%s"/>
    </logger>

    <logger name="com.singularity.ee.agent.systemagent.task.sigar.SigarAppAgentMonitor" additivity="false">
        <level value="%s"/>
        <appender-ref ref="%s"/>
    </logger>

    <root>
        <priority value="error"/>
        <appender-ref ref="%s"/>
    </root>

</log4j:configuration>
`, logLevel, appender, logLevel, appender, logLevel, appender, appender)

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_LOG_CONFIG_NAME, Namespace: infraViz.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)
	if err == nil {
		e := r.client.Delete(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to delete the old MA Log configMap. %v", e)
		}
	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load MA Log configMap. %v", err)
	}

	fmt.Printf("Recreating MA Log Config Map\n")

	cm.Name = AGENT_LOG_CONFIG_NAME
	cm.Namespace = infraViz.Namespace
	cm.Data = make(map[string]string)
	cm.Data["log4j.xml"] = string(xml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create MA Log configMap. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to re-create MA Log configMap. %v", e)
		}
	}

	fmt.Println("Configmap re-created")
	return nil
}

func (r *ReconcileInfraViz) ensureSecret(infraViz *appdynamicsv1alpha1.InfraViz) error {
	secret := &corev1.Secret{}

	key := client.ObjectKey{Namespace: infraViz.Namespace, Name: AGENT_SECRET_NAME}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Required secret %s not found. An empty secret will be created, but the clusteragent will not start until at least the 'api-user' key of the secret has a valid value", AGENT_SECRET_NAME)

		secret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      AGENT_SECRET_NAME,
				Namespace: infraViz.Namespace,
			},
		}

		secret.StringData = make(map[string]string)
		secret.StringData["api-user"] = ""
		secret.StringData["controller-key"] = ""
		secret.StringData["event-key"] = ""

		errCreate := r.client.Create(context.TODO(), secret)
		if errCreate != nil {
			fmt.Printf("Unable to create secret. %v\n", errCreate)
			return fmt.Errorf("Unable to get secret for cluster-agent. %v", errCreate)
		} else {
			fmt.Printf("Secret created. %s\n", AGENT_SECRET_NAME)
			errLoad := r.client.Get(context.TODO(), key, secret)
			if errLoad != nil {
				fmt.Printf("Unable to reload secret. %v\n", errLoad)
				return fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
			}
		}
	} else if err != nil {
		return fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
	}

	return nil
}

func (r *ReconcileInfraViz) ensureAgentService(infraViz *appdynamicsv1alpha1.InfraViz) error {
	selector := labelsForInfraViz(infraViz)
	svc := &corev1.Service{}
	key := client.ObjectKey{Namespace: infraViz.Namespace, Name: infraViz.Name}
	err := r.client.Get(context.TODO(), key, svc)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to get service for cluster-agent. %v\n", err)
	}

	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("InfraViz service not found. %v\n", err)

		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      infraViz.Name,
				Namespace: infraViz.Namespace,
				Labels:    selector,
			},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports: []corev1.ServicePort{
					{
						Name:     "biq-port",
						Protocol: corev1.ProtocolTCP,
						Port:     infraViz.Spec.BiqPort,
					},
				},
			},
		}

		if infraViz.Spec.NetVizPort > 0 {

			netVizPort := corev1.ServicePort{
				Name:     "netviz-port",
				Protocol: corev1.ProtocolTCP,
				Port:     infraViz.Spec.NetVizPort,
			}
			svc.Spec.Ports = append(svc.Spec.Ports, netVizPort)
		}
		errCreate := r.client.Create(context.TODO(), svc)
		if errCreate != nil {
			return fmt.Errorf("Failed to create infraViz agent service: %v", errCreate)
		} else {
			fmt.Printf("Infraviz service created")
		}
	}
	return nil
}

func labelsForInfraViz(infraViz *appdynamicsv1alpha1.InfraViz) map[string]string {
	return map[string]string{"name": "infraViz", "infraViz_cr": infraViz.Name}
}

func (r *ReconcileInfraViz) ensureNetVizConfig(infraViz *appdynamicsv1alpha1.InfraViz) error {
	netvizProps := fmt.Sprintf(`--
-- Copyright (c) 2019 AppDynamics Inc.
-- All rights reserved.
--
-- $Id$
package.path = './?.lua;' .. package.path
require "config_helper"

ROOT_DIR="/netviz-agent"
INSTALL_DIR=ROOT_DIR
-- Define a unique hostname for identification on controller
UNIQUE_HOST_ID = ""

--
-- NPM global configuration
-- Configurable params
-- {
--	enable_monitor = 0/1,	-- def:0, enable/disable monitoring
--	disable_filter = 0/1,	-- def:0, disable/enable language agent filtering
--	mode = KPI/Diagnostic/Advanced,	-- def:KPI
--	enable_netlib = 0/1,	-- def:0, using netlib to map appid with tuples.
--	lua_scripts_path	-- Path to lua scripts.
--	enable_fqdn = 0/1	-- def:0, enable/disable fqdn resolution of ip
-- }
--
npm_config = {
	log_destination = "file",
	log_file = "appd-netagent.log",
	debug_log_file = "agent-debug.log",
	disable_filter = 1,
	mode = "KPI",
	enable_netlib = 0,
	lua_scripts_path = ROOT_DIR .. "/scripts/netagent/lua",
	enable_fqdn = 1,
}

--
-- Webserver configuration
-- Configurable params
-- {
--	port = ,		-- Port on which to open the webserver
--	request_timeout = , -- Request timeout in ms
--	threads = ,		-- Number of threads on the webserver
-- }
--
webserver_config = {
	port = %d,
	request_timeout = 10000,
	threads = 2,
}

--
-- Packet capture configurations (multiple captures can be configured)
-- Confiurable params, there can be multiple of these.
-- {
-- 	cap_module = "pcap",		-- def:"pcap", capture module
-- 	cap_type = "device"/"file",	-- def:"device", type of capture
-- 	ifname = "",		-- def:"any", interface name/pcap filename
-- 	enable_promisc = 0/1,	-- def:0, promiscuous mode pkt capture
-- 	thinktime = ,		-- def: 100, time in msec, to sleep if no pkts
-- 	snaplen = ,		-- def:1518. pkt capture len
-- 	buflen = ,		-- def:2. pcap buffer size in MB
-- 	ppi = ,			-- def:32. pcap ppi
-- },
--
capture = {
	-- first capture interface
	{
		cap_module = "pcap",
		cap_type = "device",
		ifname = "any",
		thinktime = 25,
		buflen = 48,
--		filter = "",
	},
--[[	{
		cap_module = "pcap",
		cap_type = "device",
		ifname = "en0",
	},
--]]
}

--
-- IP configuration
-- ip_config = {
--	expire_timeout = ,	-- Mins after which we expire ip metadata
--	retry_count = ,		-- No of tries to resolve fqdn for ip
-- }
ip_config = {
	expire_interval = 20,
	retry_count = 5,
}

--
-- DPI configuration
-- Configurable params
-- {
--	max_flows = ,	-- Max number of flows per fg to DPI at any given time.
--	max_data = ,	-- Max mega bytes to DPI per flow.
--	max_depth = ,	-- Max bytes to DPI in a packet
--	max_callchains = , -- Max callchains to store for a flowgroup
--	max_cc_perflow = , -- Max number of call chains to look for in each flow
-- }
--
dpi_config = {
	max_flows = 10,
	max_data = 4,
	max_depth = 4096,
	max_callchains_in_fg = 32,
	max_callchains_in_flow = 2,
}

-- Configurations for application service ports
-- {
--	ports = ,	-- Comma separated list of application service
--			   ports greater than 32000. Example
--			   ports = "40000, 41000, 42000"
-- }
--[[
application_service_ports = {
	ports = "",
}
--]]

--
-- Export data from network agent configuration/tunnables
-- Configurable params, there can be multiple of these.
-- {
-- 	exportype = "file"/"remote",	-- type of export mechanism
-- 	statsfile = "",			-- filename for stats export
-- 	metricsfile = "", 		-- filename for metrics export
-- 	serialization = "pb",		-- pb/capnp, serialization module
-- 	transport = "zmq", 		-- def:"zmq", transport module
-- 	zmqdest = "", 			-- dest peer for zmq
--  },
--
export_config = {
	-- file export
	{
		exporttype = "file",
		statsfile = "agent-stats.log",
		metricsfile = "agent-metrics.log",
		eventsfile =  "agent-events.log",
		snapshotsfile = "agent-snapshots.log",
		metadatafile = "agent-metadata.log",
	},
}

-- Plugin interface configuration.
-- List of interfaces to be monitored by supported plugins.
-- Configurable params, there can be multiple of these.
-- {
-- 	interface = "eth0",	-- def: "eth0", interface name
-- }
plugin_if_config = {
--[[
	{interface = "eth0"},
--]]
}

-- Plugin process configuration.
-- List of processes to be monitored by supported plugins.
-- Configurable params, there can be multiple of these.
-- {
--	process = "",		-- def: "appd-netagent", process name
-- }
plugin_proc_config = {
	{process = "appd-netagent"},
}

-- metadata to pass to pass the agent metadata specific params
system_metadata = {
	unique_host_id = UNIQUE_HOST_ID,
	install_dir = INSTALL_DIR,
	install_time = get_last_update_time(),
}
`, infraViz.Spec.NetVizPort)

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_NETVIZ_CONFIG_NAME, Namespace: infraViz.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)
	//	if err == nil {
	//		e := r.client.Delete(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to delete the old Netviz configMap. %v", e)
	//		}
	//	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Netviz configMap. %v", err)
	}

	fmt.Printf("Recreating Netviz Config Map\n")

	cm.Name = AGENT_NETVIZ_CONFIG_NAME
	cm.Namespace = infraViz.Namespace
	cm.Data = make(map[string]string)
	cm.Data["agent_config.lua"] = string(netvizProps)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Netviz configMap. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to re-create Netviz configMap. %v", e)
		}
	}

	fmt.Println("Netviz Configmap re-created")
	return nil
}
