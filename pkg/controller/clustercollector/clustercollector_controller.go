package clustercollector

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

const (
	OLD_SPEC string = "cluster-collector-spec"
	ClUSTER_MON_CONFIG_NAME string = "cluster-collector-config"
	INFRA_AGENT_CONFIG_NAME string = "infra-agent-config"
)

var log = logf.Log.WithName("controller_clustercollector")

// Add creates a new Clustercollector Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClustercollector{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clustercollector-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Clustercollector
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Clustercollector{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner Clustercollector
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clustercollector{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileClustercollector{}

// ReconcileClustercollector reconciles a Clustercollector object
type ReconcileClustercollector struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileClustercollector) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Clustercollector...")

	clusterCollector := &appdynamicsv1alpha1.Clustercollector{}
	err := r.client.Get(context.TODO(), request.NamespacedName, clusterCollector)
	reqLogger.Info("Retrieved cluster collector.", "Image", clusterCollector.Spec.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			reqLogger.Info("Cluster Collector resource not found. The object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Cluster Collector")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Cluster Collector spec exists. Checking the corresponding deployment...")
	// Check if the collector already exists in the namespace
	existingDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: clusterCollector.Name, Namespace: clusterCollector.Namespace}, existingDeployment)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Removing the old instance of the configMap...")
		reqLogger.Info("Cluster Collector deployment does not exist. Creating...")

		// Define a new deployment for the cluster collector
		econfig := r.ensureConfigMap(clusterCollector)
		if econfig != nil {
			reqLogger.Error(econfig, "Failed to create new Cluster Collector due to config map", "Deployment.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
			return reconcile.Result{}, econfig
		}
		dep := r.newCollectorDeployment(clusterCollector)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deployment created successfully. Done")
		r.updateStatus(clusterCollector)
		return reconcile.Result{}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Cluster Collector deployment exists. Checking for deltas with the current state...")

	reqLogger.Info("Retrieving agent config map", "Deployment.Namespace", clusterCollector.Namespace)
	econfig := r.ensureConfigMap(clusterCollector)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain cluster agent config map", "Deployment.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
		return reconcile.Result{}, econfig
	}
	breaking, updateDeployment := r.hasBreakingChanges(clusterCollector, existingDeployment)

	if breaking {
		fmt.Println("Breaking changes detected. Restarting the cluster collector pod...")

		saveOrUpdateClusterCollectorSpecAnnotation(clusterCollector, existingDeployment)
		errUpdate := r.client.Update(context.TODO(), existingDeployment)
		if errUpdate != nil {
			reqLogger.Error(errUpdate, "Failed to update cluster collector", "clusterCollector.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
			return reconcile.Result{}, errUpdate
		}

		errRestart := r.restartCollector(clusterCollector)
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart cluster collector", "clusterCollector.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
			return reconcile.Result{}, errRestart
		}
	} else if updateDeployment {
		fmt.Println("Breaking changes detected. Updating the the cluster collector deployment...")
		err = r.client.Update(context.TODO(), existingDeployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update Clustercollector Deployment", "Deployment.Namespace", existingDeployment.Namespace, "Deployment.Name", existingDeployment.Name)
			return reconcile.Result{}, err
		}
	} else {

		reqLogger.Info("No breaking changes.", "clusterCollector.Namespace", clusterCollector.Namespace)

		statusErr := r.updateStatus(clusterCollector)
		if statusErr == nil {
			reqLogger.Info("Status updated. Exiting reconciliation loop.")
		} else {
			reqLogger.Info("Status not updated. Exiting reconciliation loop.")
		}
		return reconcile.Result{}, nil

	}

	reqLogger.Info("Exiting reconciliation loop.")
	return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *ReconcileClustercollector) updateStatus(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	clusterCollector.Status.LastUpdateTime = metav1.Now()

	if errInstance := r.client.Update(context.TODO(), clusterCollector); errInstance != nil {
		return fmt.Errorf("Unable to update clustercollector instance. %v", errInstance)
	}
	log.Info("Clustercollector instance updated successfully", "clusterCollector.Namespace", clusterCollector.Namespace, "Date", clusterCollector.Status.LastUpdateTime)

	err := r.client.Status().Update(context.TODO(), clusterCollector)
	if err != nil {
		log.Error(err, "Failed to update cluster collector status", "clusterCollector.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
	} else {
		log.Info("Clustercollector status updated successfully", "clusterCollector.Namespace", clusterCollector.Namespace, "Date", clusterCollector.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileClustercollector) hasBreakingChanges(clusterCollector *appdynamicsv1alpha1.Clustercollector, existingDeployment *appsv1.Deployment) (bool, bool) {
	fmt.Println("Checking for breaking changes...")
	if clusterCollector.Spec.Image != "" && existingDeployment.Spec.Template.Spec.Containers[0].Image != clusterCollector.Spec.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDeployment.Spec.Template.Spec.Containers[0].Image, clusterCollector.Spec.Image)
		existingDeployment.Spec.Template.Spec.Containers[0].Image = clusterCollector.Spec.Image
		return false, true
	}
	return false, false
}

func (r *ReconcileClustercollector) newCollectorDeployment(clusterCollector *appdynamicsv1alpha1.Clustercollector) *appsv1.Deployment {
	if clusterCollector.Spec.Image == "" {
		clusterCollector.Spec.Image = "vikyath/infra-agent-cluster-collector:latest"
	}

	if clusterCollector.Spec.ServiceAccountName == "" {
		clusterCollector.Spec.ServiceAccountName = "appdynamics-operator"
	}

	fmt.Printf("Building deployment spec for image %s\n", clusterCollector.Spec.Image)
	ls := labelsForClusterCollector(clusterCollector)
	var replicas int32 = 1
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterCollector.Name,
			Namespace: clusterCollector.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: clusterCollector.Spec.ServiceAccountName,
					Containers: []corev1.Container{{
						Env: []corev1.EnvVar{
							{
								Name: "APPDYNAMICS_AGENT_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "NODE_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
						},
						Image:           clusterCollector.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "cluster-collector",
						Resources:       clusterCollector.Spec.Resources,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name: "clustermon-config",
								MountPath: "/opt/appdynamics/InfraAgent/collectors/clustermon.conf",
								SubPath: "clustermon.conf",
							},
							{
								Name: "infraagent-config",
								MountPath: "/opt/appdynamics/InfraAgent/agent.conf",
								SubPath: "agent.conf",
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "clustermon-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: ClUSTER_MON_CONFIG_NAME},
								},
							},
						},
						{
							Name: "infraagent-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: INFRA_AGENT_CONFIG_NAME},
								},
							},
						},
					},
				},
			},
		},
	}

	//save the new spec in annotations
	saveOrUpdateClusterCollectorSpecAnnotation(clusterCollector, dep)

	// Set Cluster collector instance as the owner and controller
	controllerutil.SetControllerReference(clusterCollector, dep, r.scheme)
	return dep
}
func (r *ReconcileClustercollector) ensureConfigMap(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
    setClusterCollectorConfigDefaults(clusterCollector)
    setInfraAgentConfigsDefaults(clusterCollector)
    err := r.ensureClusterCollectorConfig(clusterCollector)
    if err != nil {
    	return err
	}
	err = r.ensureInfraAgentConfig(clusterCollector)
	if err != nil {
		return err
	}
	return nil
}

func setClusterCollectorConfigDefaults(clusterCollector *appdynamicsv1alpha1.Clustercollector) {
	if clusterCollector.Spec.LogLevel == "" {
		clusterCollector.Spec.LogLevel = "INFO"
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
		clusterCollector.Spec.SystemConfigs.ClientLibRecvUrl = "tcp://127.0.0.1:42387"
	}
	if clusterCollector.Spec.SystemConfigs.LogLevel == "" {
		clusterCollector.Spec.SystemConfigs.LogLevel = "INFO"
	}
	if clusterCollector.Spec.SystemConfigs.DebugPort == "" {
		clusterCollector.Spec.SystemConfigs.DebugPort = "39987"
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

func (r *ReconcileClustercollector) ensureInfraAgentConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	errVal, controllerDns, port, sslEnabled := validateControllerUrl(clusterCollector.Spec.ControllerUrl)
	if errVal != nil {
		return errVal
	}
	portVal := strconv.Itoa(int(port))

	yml := fmt.Sprintf(`controller-host: %s
controller-port: %s
controller-account-name: %s
controller-ssl-enabled: %s
enabled: %t
controller-access-key: %s
controller-lib-socket-url: %s
collector-lib-port: %s
http-client-timeout: %d
http-client-basic-auth-enabled: %t
configuration-change-scan-period: %d
configuration-stale-grace-period: %d
debug-port: %s
client-lib-send-url: %s
client-lib-recv-url: %s
log-level: %s
debug-enabled: %t`, controllerDns, portVal, clusterCollector.Spec.Account, sslEnabled, true, clusterCollector.Spec.AccessSecret,
clusterCollector.Spec.SystemConfigs.CollectorLibSocketUrl, clusterCollector.Spec.SystemConfigs.CollectorLibPort,
clusterCollector.Spec.SystemConfigs.HttpClientTimeOut, clusterCollector.Spec.SystemConfigs.HttpBasicAuthEnabled,
clusterCollector.Spec.SystemConfigs.ConfigChangeScanPeriod, clusterCollector.Spec.SystemConfigs.ConfigStaleGracePeriod,
clusterCollector.Spec.SystemConfigs.DebugPort, clusterCollector.Spec.SystemConfigs.ClientLibSendUrl,
clusterCollector.Spec.SystemConfigs.ClientLibRecvUrl, clusterCollector.Spec.SystemConfigs.LogLevel, clusterCollector.Spec.SystemConfigs.DebugEnabled)
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: INFRA_AGENT_CONFIG_NAME, Namespace: clusterCollector.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load infra agent configMap. %v", err)
	}



	cm.Name = INFRA_AGENT_CONFIG_NAME
	cm.Namespace = clusterCollector.Namespace
	cm.Data = make(map[string]string)
	cm.Data["agent.conf"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create infra agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update infra agent configMap. %v", e)
		}
		fmt.Println("Infra Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileClustercollector) ensureClusterCollectorConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {

	yml := fmt.Sprintf(`clusterName: %s
nsToMonitor: %s
nsToExclude: %s
clusterMonitoringEnabled: %t`, clusterCollector.Spec.ClusterName, clusterCollector.Spec.NsToMonitorRegex, clusterCollector.Spec.NsToExcludeRegex,
clusterCollector.Spec.ClusterMonEnabled)
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ClUSTER_MON_CONFIG_NAME, Namespace: clusterCollector.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load clustermon configMap. %v", err)
	}



	cm.Name = ClUSTER_MON_CONFIG_NAME
	cm.Namespace = clusterCollector.Namespace
	cm.Data = make(map[string]string)
    cm.Data["clustermon.conf"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create clustermon configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update clustermon configMap. %v", e)
		}
		fmt.Println("Cluster collector Configmap updated")
	}

	return nil

}

func saveOrUpdateClusterCollectorSpecAnnotation(clusterCollector *appdynamicsv1alpha1.Clustercollector, dep *appsv1.Deployment) {
	jsonObj, e := json.Marshal(clusterCollector)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "clusterCollector.Namespace", clusterCollector.Namespace, "clusterCollector.Name", clusterCollector.Name)
	} else {
		if dep.Annotations == nil {
			dep.Annotations = make(map[string]string)
		}
		dep.Annotations[OLD_SPEC] = string(jsonObj)
	}
}

func (r *ReconcileClustercollector) restartCollector(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForClusterCollector(clusterCollector))
	listOps := &client.ListOptions{
		Namespace:     clusterCollector.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve cluster-collector pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = r.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete cluster-collector pod. %v", err)
	}
	return nil
}

func labelsForClusterCollector(clusterCollector *appdynamicsv1alpha1.Clustercollector) map[string]string {
	return map[string]string{"name": "clusterCollector", "clusterCollector_cr": clusterCollector.Name}
}
