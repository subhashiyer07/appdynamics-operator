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
	OLD_SPEC string = "host-collector-spec"
	HOST_COLLECTOR_NAME string = "k8s-host-collector"
	CONTAINER_COLLECTOR string = "Container Monitor"
	TYPE_COLLECTOR string = "Collector"
	CONTAINER_COLLECTOR_PATH string = "./collectors/containermon-collector-linux-amd64"
	CONTAINER_CONFIG_NAME string = "container-collector-config"
	INFRA_AGENT_NAME string = "Infra Structure Agent"
	INFRA_AGENT_CONFIG_NAME string = "infra-agent-config"
)

var log = logf.Log.WithName("controller_hostcollector")

// Add creates a new Hostcollector Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileHostcollector{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("hostcollector-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Hostcollector
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Clustercollector{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource DaemonSet and requeue the owner Hostcollector
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clustercollector{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileHostcollector{}

// ReconcileHostcollector reconciles a Hostcollector object
type ReconcileHostcollector struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileHostcollector) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Hostcollector...")

	hostCollector := &appdynamicsv1alpha1.Clustercollector{}
	err := r.client.Get(context.TODO(), request.NamespacedName, hostCollector)
	reqLogger.Info("Retrieved host collector.", "Image", hostCollector.Spec.HostCollector.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			reqLogger.Info("Host Collector resource not found. The object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Host Collector")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Host Collector spec exists. Checking the corresponding DaemonSet...")
	// Check if the collector already exists in the namespace
	existingDaemonSet := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: hostCollector.Name, Namespace: hostCollector.Namespace}, existingDaemonSet)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Removing the old instance of the configMap...")
		reqLogger.Info("Host Collector daemonSet does not exist. Creating...")

		// Define a new DaemonSet for the host collector
		daemonSet := r.newCollectorDaemonSet(hostCollector)
		reqLogger.Info("Creating a new DaemonSet", "DaemonSet.Namespace", daemonSet.Namespace, "DaemonSet.Name", daemonSet.Name)
		err = r.client.Create(context.TODO(), daemonSet)
		if err != nil {
			reqLogger.Error(err, "Failed to create new DaemonSet", "DaemonSet.Namespace", daemonSet.Namespace, "DaemonSet.Name", daemonSet.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("DaemonSet created successfully. Done")
		_ = r.updateStatus(hostCollector)
		return reconcile.Result{}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get DaemonSet")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Host Collector daemonSet exists. Checking for deltas with the current state...")

	breaking, updateDaemonSet := r.hasBreakingChanges(hostCollector, existingDaemonSet)

	if breaking {
		fmt.Println("Breaking changes detected. Restarting the host collector pod...")

		saveOrUpdateHostCollectorSpecAnnotation(hostCollector, existingDaemonSet)
		errUpdate := r.client.Update(context.TODO(), existingDaemonSet)
		if errUpdate != nil {
			reqLogger.Error(errUpdate, "Failed to update host collector", "hostCollector.Namespace", hostCollector.Namespace, "DaemonSet.Name", hostCollector.Name)
			return reconcile.Result{}, errUpdate
		}

		errRestart := r.restartCollector(hostCollector)
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart host collector", "hostCollector.Namespace", hostCollector.Namespace, "DaemonSet.Name", hostCollector.Name)
			return reconcile.Result{}, errRestart
		}
	} else if updateDaemonSet {
		fmt.Println("Breaking changes detected. Updating the the host collector daemonSet...")
		err = r.client.Update(context.TODO(), existingDaemonSet)
		if err != nil {
			reqLogger.Error(err, "Failed to update Hostcollector DaemonSet", "DaemonSet.Namespace", existingDaemonSet.Namespace, "DaemonSet.Name", existingDaemonSet.Name)
			return reconcile.Result{}, err
		}
	} else {

		reqLogger.Info("No breaking changes.", "hostCollector.Namespace", hostCollector.Namespace)

		statusErr := r.updateStatus(hostCollector)
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

func (r *ReconcileHostcollector) updateStatus(hostCollector *appdynamicsv1alpha1.Clustercollector) error {
	hostCollector.Status.LastUpdateTime = metav1.Now()

	if errInstance := r.client.Update(context.TODO(), hostCollector); errInstance != nil {
		return fmt.Errorf("Unable to update hostcollector instance. %v", errInstance)
	}
	log.Info("Hostcollector instance updated successfully", "hostCollector.Namespace", hostCollector.Namespace, "Date", hostCollector.Status.LastUpdateTime)

	err := r.client.Status().Update(context.TODO(), hostCollector)
	if err != nil {
		log.Error(err, "Failed to update host collector status", "hostCollector.Namespace", hostCollector.Namespace, "DaemonSet.Name", hostCollector.Name)
	} else {
		log.Info("Hostcollector status updated successfully", "hostCollector.Namespace", hostCollector.Namespace, "Date", hostCollector.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileHostcollector) hasBreakingChanges(hostCollector *appdynamicsv1alpha1.Clustercollector, existingDaemonSet *appsv1.DaemonSet) (bool, bool) {
	fmt.Println("Checking for breaking changes...")
	if hostCollector.Spec.HostCollector.Image != "" && existingDaemonSet.Spec.Template.Spec.Containers[0].Image != hostCollector.Spec.HostCollector.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDaemonSet.Spec.Template.Spec.Containers[0].Image, hostCollector.Spec.HostCollector.Image)
		existingDaemonSet.Spec.Template.Spec.Containers[0].Image = hostCollector.Spec.HostCollector.Image
		return false, true
	}

	return false, false
}

func (r *ReconcileHostcollector) newCollectorDaemonSet(hostCollector *appdynamicsv1alpha1.Clustercollector) *appsv1.DaemonSet {
	trueVal := true
	if hostCollector.Spec.HostCollector.Image == "" {
		hostCollector.Spec.HostCollector.Image = "vikyath/host-collector:latest"
	}

	if hostCollector.Spec.HostCollector.ServiceAccountName == "" {
		hostCollector.Spec.HostCollector.ServiceAccountName = "appdynamics-operator"
	}

	fmt.Printf("Building DaemonSet spec for image %s\n", hostCollector.Spec.Image)
	ls := labelsForHostCollector(hostCollector)
	ds := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      HOST_COLLECTOR_NAME,
			Namespace: hostCollector.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: hostCollector.Spec.HostCollector.ServiceAccountName,
					Containers: []corev1.Container{{
						Image:           hostCollector.Spec.HostCollector.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "host-collector",
						Resources:       hostCollector.Spec.HostCollector.Resources,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &trueVal,
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "proc",
							MountPath: "/host/proc",
							ReadOnly:  true,
						}, {
							Name:      "var-run",
							MountPath: "/var/run",
						}, {
							Name:      "sys",
							MountPath: "/sys",
							ReadOnly:  true,
						}, {
							Name:      "root",
							MountPath: "/rootfs",
							ReadOnly:  true,
						}, {
							Name:      "var-lib-docker",
							MountPath: "/var/lib/docker/",
							ReadOnly:  true,
						}, {
							Name: "infraagent-config",
							MountPath: "/opt/appdynamics/InfraAgent/agent.conf",
							SubPath: "agent.conf",
						},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "proc",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/proc"},
						},
					}, {
						Name: "var-run",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/var/run"},
						},
					}, {
						Name: "sys",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/sys"},
						},
					}, {
						Name: "root",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/"},
						},
					}, {
						Name: "var-lib-docker",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/docker/"},
						},
					},{
						Name: "infraagent-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: INFRA_AGENT_CONFIG_NAME},
							},
						},
					}},
				},
			},
		},
	}

	//save the new spec in annotations
	saveOrUpdateHostCollectorSpecAnnotation(hostCollector, ds)

	// Set Host collector instance as the owner and controller
	_ = controllerutil.SetControllerReference(hostCollector, ds, r.scheme)
	return ds
}

func saveOrUpdateHostCollectorSpecAnnotation(hostCollector *appdynamicsv1alpha1.Clustercollector, ds *appsv1.DaemonSet) {
	jsonObj, e := json.Marshal(hostCollector.Spec.HostCollector)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "hostCollector.Namespace", hostCollector.Namespace, "hostCollector.Name", hostCollector.Name)
	} else {
		if ds.Annotations == nil {
			ds.Annotations = make(map[string]string)
		}
		ds.Annotations[OLD_SPEC] = string(jsonObj)
	}
}

func (r *ReconcileHostcollector) restartCollector(hostCollector *appdynamicsv1alpha1.Clustercollector) error {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForHostCollector(hostCollector))
	listOps := &client.ListOptions{
		Namespace:     hostCollector.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve host-collector pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = r.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete host-collector pod. %v", err)
	}
	return nil
}

func labelsForHostCollector(hostCollector *appdynamicsv1alpha1.Clustercollector) map[string]string {
	return map[string]string{"name": "hostCollector", "hostCollector_cr": HOST_COLLECTOR_NAME}
}

func (r *ReconcileHostcollector) ensureConfigMap(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	/*setContainerMonConfigDefaults(clusterCollector)
	setServerMonConfigDefaults(clusterCollector)
	*/
	setInfraAgentConfigsDefaults(clusterCollector)
	err := r.ensureContainerCollectorConfig(clusterCollector)
	if err != nil {
		return err
	}
	err = r.ensureInfraAgentConfig(clusterCollector)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileHostcollector) ensureContainerCollectorConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {

	yml := fmt.Sprintf(`name: %s
type: %s
version: %s
log-level: %s
path: %s
enabled: %t
exporter-address: %s
exporter-port: %d`, CONTAINER_COLLECTOR, TYPE_COLLECTOR, strings.Split(clusterCollector.Spec.HostCollector.Image,":")[1] , clusterCollector.Spec.LogLevel,
		CONTAINER_COLLECTOR_PATH, true, clusterCollector.Spec.ExporterAddress, clusterCollector.Spec.ExporterPort)
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: CONTAINER_CONFIG_NAME, Namespace: clusterCollector.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load clustermon configMap. %v", err)
	}



	cm.Name = CONTAINER_CONFIG_NAME
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

func (r *ReconcileHostcollector) ensureInfraAgentConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	errVal, controllerDns, port, sslEnabled := validateControllerUrl(clusterCollector.Spec.ControllerUrl)
	if errVal != nil {
		return errVal
	}
	portVal := strconv.Itoa(int(port))

	yml := fmt.Sprintf(`name: %s 
controller-host: %s
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
debug-enabled: %t`, INFRA_AGENT_NAME, controllerDns, portVal, clusterCollector.Spec.Account, sslEnabled, true, clusterCollector.Spec.AccessSecret,
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