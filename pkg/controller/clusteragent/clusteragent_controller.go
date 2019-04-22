package clusteragent

import (
	"context"
	"fmt"

	"encoding/json"
	//	"time"

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

var log = logf.Log.WithName("controller_clusteragent")

const (
	AGENT_SECRET_NAME string = "cluster-agent-secret"
	AGENt_CONFIG_NAME string = "cluster-agent-config"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Clusteragent Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusteragent{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusteragent-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Clusteragent
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Clusteragent{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner Clusteragent
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clusteragent{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileClusteragent{}

// ReconcileClusteragent reconciles a Clusteragent object
type ReconcileClusteragent struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileClusteragent) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Clusteragent...")

	// Fetch the Clusteragent instance
	clusterAgent := &appdynamicsv1alpha1.Clusteragent{}
	err := r.client.Get(context.TODO(), request.NamespacedName, clusterAgent)
	reqLogger.Info("Retrieved cluster agent.", "Image", clusterAgent.Spec.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Cluster Agent resource not found. The object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Cluster Agent")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Cluster agent spec exists. Checking the corresponding deployment...")
	// Check if the agent already exists in the namespace
	existingDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: clusterAgent.Name, Namespace: clusterAgent.Namespace}, existingDeployment)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Cluster agent deployment does not exist. Creating...")
		reqLogger.Info("Checking the secret")
		secret, esecret := r.ensureSecret(clusterAgent)
		if esecret != nil {
			reqLogger.Error(esecret, "Failed to create new Cluster Agent due to secret", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, esecret
		}
		reqLogger.Info("Checking the config map")
		_, _, econfig := r.ensureConfigMap(clusterAgent, secret, true)
		if econfig != nil {
			reqLogger.Error(econfig, "Failed to create new Cluster Agent due to config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, econfig
		}
		reqLogger.Info("Creating service...\n")
		_, esvc := r.ensureAgentService(clusterAgent)
		if esvc != nil {
			reqLogger.Error(esvc, "Failed to create new Cluster Agent due to service", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, esvc
		}
		// Define a new deployment for the cluster agent
		dep := r.newAgentDeployment(clusterAgent)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deployment created successfully. Done")
		r.updateStatus(clusterAgent)
		return reconcile.Result{}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Cluster agent deployment exists. Checking for deltas with the current state...")
	// Ensure the deployment spec matches the new spec
	// Differentiate between breaking changes and benign updates
	// Check if secret has been recreated. if yes, restart pod
	secret, errsecret := r.ensureSecret(clusterAgent)
	if errsecret != nil {
		reqLogger.Error(errsecret, "Failed to get cluster agent config secret", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, errsecret
	}

	reqLogger.Info("Retrieving agent config map", "Deployment.Namespace", clusterAgent.Namespace)
	cm, bag, econfig := r.ensureConfigMap(clusterAgent, secret, false)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain cluster agent config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, econfig
	}

	breaking, _ := r.hasBreakingChanges(clusterAgent, bag, secret)

	if breaking {
		fmt.Println("Breaking changes detected. Restarting the cluster agent pod...")
		errRestart := r.restartAgent(clusterAgent)
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart cluster agent", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, errRestart
		}
	} else {
		//update the configMap
		reqLogger.Info("No breaking changes. Reconciling the config map...", "clusterAgent.Namespace", clusterAgent.Namespace)
		errMap := r.updateMap(cm, clusterAgent, secret, false)
		if errMap != nil {
			reqLogger.Error(errMap, "Issues when reconciling the config map...", "clusterAgent.Namespace", clusterAgent.Namespace)
			return reconcile.Result{}, errMap
		} else {
			statusErr := r.updateStatus(clusterAgent)
			if statusErr == nil {
				reqLogger.Info("Status updated. Exiting reconciliation loop.")
			} else {
				reqLogger.Info("Status not updated. Exiting reconciliation loop.")
			}
			return reconcile.Result{}, nil
		}
	}

	reqLogger.Info("Exiting reconciliation loop.")
	return reconcile.Result{}, nil
}

func (r *ReconcileClusteragent) updateStatus(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	clusterAgent.Status.LastUpdateTime = metav1.Now()
	err := r.client.Update(context.TODO(), clusterAgent)
	if err != nil {
		log.Error(err, "Failed to update cluster agent status", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
	} else {
		log.Info("ClusterAgent status updated successfully", "clusterAgent.Namespace", clusterAgent.Namespace, "Date", clusterAgent.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileClusteragent) hasBreakingChanges(clusterAgent *appdynamicsv1alpha1.Clusteragent, bag *appdynamicsv1alpha1.AppDBag, secret *corev1.Secret) (bool, bool) {
	breaking := false
	benign := true

	fmt.Println("Checking for breaking changes...")

	if bag.SecretVersion != secret.ResourceVersion {
		fmt.Printf("SecretVersion has changed: %s		%s\n", bag.SecretVersion, secret.ResourceVersion)
		return true, benign
	}

	if bag.SecretVersion != secret.ResourceVersion || clusterAgent.Spec.ControllerUrl != bag.ControllerUrl {
		fmt.Printf("ControllerUrl has changed: %s		%s\n", bag.ControllerUrl, clusterAgent.Spec.ControllerUrl)
		return true, benign
	}

	if clusterAgent.Spec.Account != "" && clusterAgent.Spec.Account != bag.Account {
		fmt.Printf("AccountName has changed: %s		%s\n", bag.Account, clusterAgent.Spec.Account)
		return true, benign
	}

	if clusterAgent.Spec.GlobalAccount != "" && clusterAgent.Spec.GlobalAccount != bag.GlobalAccount {
		fmt.Printf("GlobalAccountName has changed: %s		%s\n", bag.GlobalAccount, clusterAgent.Spec.GlobalAccount)
		return true, benign
	}
	if clusterAgent.Spec.AppName != "" && clusterAgent.Spec.AppName != bag.AppName {
		fmt.Printf("AppName has changed: %s		%s\n", bag.AppName, clusterAgent.Spec.AppName)
		return true, benign
	}

	if clusterAgent.Spec.EventServiceUrl != "" && clusterAgent.Spec.EventServiceUrl != bag.EventServiceUrl {
		fmt.Printf("EventServiceUrl has changed: %s		%s\n", bag.EventServiceUrl, clusterAgent.Spec.EventServiceUrl)
		return true, benign
	}
	if clusterAgent.Spec.SystemSSLCert != "" && clusterAgent.Spec.SystemSSLCert != bag.SystemSSLCert {
		fmt.Printf("SystemSSLCert has changed: %s		%s\n", bag.SystemSSLCert, clusterAgent.Spec.SystemSSLCert)
		return true, benign
	}

	if clusterAgent.Spec.AgentSSLCert != "" && clusterAgent.Spec.AgentSSLCert != bag.AgentSSLCert {
		fmt.Printf("AgentSSLCert has changed: %s		%s\n", bag.AgentSSLCert, clusterAgent.Spec.AgentSSLCert)
		return true, benign
	}

	return breaking, benign
}

func (r *ReconcileClusteragent) ensureSecret(clusterAgent *appdynamicsv1alpha1.Clusteragent) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: AGENT_SECRET_NAME}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil {
		return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
	}

	return secret, nil
}

func (r *ReconcileClusteragent) ensureAgentService(clusterAgent *appdynamicsv1alpha1.Clusteragent) (*corev1.Service, error) {
	selector := labelsForClusteragent(clusterAgent)
	svc := &corev1.Service{}
	key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: clusterAgent.Name}
	err := r.client.Get(context.TODO(), key, svc)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("Unable to get service for cluster-agent. %v\n", err)
	}

	if err != nil && errors.IsNotFound(err) {
		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterAgent.Name,
				Namespace: clusterAgent.Namespace,
				Labels:    selector,
			},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports: []corev1.ServicePort{
					{
						Name:     "web-port",
						Protocol: corev1.ProtocolTCP,
						Port:     8989,
					},
				},
			},
		}
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			return nil, fmt.Errorf("Failed to create cluster agent service: %v", err)
		}
	}
	return svc, nil
}

func (r *ReconcileClusteragent) ensureConfigMap(clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret, create bool) (*corev1.ConfigMap, *appdynamicsv1alpha1.AppDBag, error) {
	cm := &corev1.ConfigMap{}
	var bag appdynamicsv1alpha1.AppDBag
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster-agent-config", Namespace: clusterAgent.Namespace}, cm)
	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, fmt.Errorf("Failed to load configMap cluster-agent-config. %v", err)
	}
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Congigmap not found. Creating...\n")
		//configMap does not exist. Create
		cm.Name = "cluster-agent-config"
		cm.Namespace = clusterAgent.Namespace
		errMap := r.updateMap(cm, clusterAgent, secret, create)
		if errMap != nil {
			return nil, nil, errMap
		}
	}
	if err == nil {
		//deserialize the map into the property bag
		jsonData := cm.Data["cluster-agent-config.json"]
		jsonErr := json.Unmarshal([]byte(jsonData), &bag)
		if jsonErr != nil {
			return nil, nil, fmt.Errorf("Enable to retrieve the configMap. Cannot deserialize. %v", jsonErr)
		}
	}

	return cm, &bag, nil

}

func (r *ReconcileClusteragent) updateMap(cm *corev1.ConfigMap, clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret, create bool) error {
	bag := appdynamicsv1alpha1.GetDefaultProperties()

	reconcileBag(bag, clusterAgent, secret)

	//	bag.DeploysToDashboard = make([]string, len(clusterAgent.Spec.DashboardTiers))
	//	copy(bag.DeploysToDashboard, clusterAgent.Spec.DashboardTiers)

	data, errJson := json.Marshal(bag)
	if errJson != nil {
		return fmt.Errorf("Enable to create configMap. Cannot serialize the config Bag. %v", errJson)
	}
	cm.Data = make(map[string]string)
	cm.Data["cluster-agent-config.json"] = string(data)
	var e error
	if create {
		e = r.client.Create(context.TODO(), cm)
		fmt.Printf("Configmap created. Error = %v\n", e)
	} else {
		e = r.client.Update(context.TODO(), cm)
		fmt.Printf("Configmap updated. Error = %v\n", e)
	}

	if e != nil {
		return fmt.Errorf("Failed to save configMap cluster-agent-config. %v", e)
	}
	return nil
}

func (r *ReconcileClusteragent) newAgentDeployment(clusterAgent *appdynamicsv1alpha1.Clusteragent) *appsv1.Deployment {
	if clusterAgent.Spec.Image == "" {
		clusterAgent.Spec.Image = "appdynamics/cluster-agent-operator:latest"
	}
	fmt.Printf("Building deployment spec for image %s\n", clusterAgent.Spec.Image)
	ls := labelsForClusteragent(clusterAgent)
	var replicas int32 = 1
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterAgent.Name,
			Namespace: clusterAgent.Namespace,
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
					ServiceAccountName: "appdynamics-operator",
					Containers: []corev1.Container{{
						Env: []corev1.EnvVar{
							{
								Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
										Key:                  "controller-key",
									},
								},
							},
							{
								Name: "APPDYNAMICS_EVENT_ACCESS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
										Key:                  "event-key",
									},
								},
							},
							{
								Name: "APPDYNAMICS_REST_API_CREDENTIALS",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
										Key:                  "api-user",
									},
								},
							},
							{
								Name: "APPDYNAMICS_AGENT_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						Image:           clusterAgent.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Name:            "cluster-agent",
						Resources:       clusterAgent.Spec.Resources,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8989,
							Protocol:      corev1.ProtocolTCP,
							Name:          "web-port",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "agent-config",
							MountPath: "/opt/appdynamics/config/",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "agent-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cluster-agent-config",
								},
							},
						},
					}},
				},
			},
		},
	}
	// Set Cluster Agent instance as the owner and controller
	controllerutil.SetControllerReference(clusterAgent, dep, r.scheme)
	return dep
}

func (r *ReconcileClusteragent) restartAgent(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForClusteragent(clusterAgent))
	listOps := &client.ListOptions{
		Namespace:     clusterAgent.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve cluster-agent pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = r.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete cluster-agent pod. %v", err)
	}
	return nil
}

func labelsForClusteragent(clusterAgent *appdynamicsv1alpha1.Clusteragent) map[string]string {
	return map[string]string{"name": "clusterAgent", "clusterAgent_cr": clusterAgent.Name}
}
