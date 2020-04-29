package instrumentation

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

var log = logf.Log.WithName("controller_instrumentation")

const (
	AGENT_SECRET_NAME           string = "cluster-agent-secret"
	AGENT_PROXY_SECRET_NAME     string = "cluster-agent-proxy-secret"
	AGENT_CONFIG_NAME           string = "cluster-agent-config"
	AGENT_MON_CONFIG_NAME       string = "cluster-agent-mon"
	INSTRUMENTATION_CONFIG_NAME string = "instrumentation-config"
	AGENT_LOG_CONFIG_NAME       string = "cluster-agent-log"
	AGENT_SSL_CONFIG_NAME       string = "appd-agent-ssl-config"
	AGENT_SSL_CRED_STORE_NAME   string = "appd-agent-ssl-store"
	OLD_SPEC                    string = "cluster-agent-spec"
	WHITELISTED                 string = "whitelisted"
	BLACKLISTED                 string = "blacklisted"

	ENV_INSTRUMENTATION string = "Env"
	JAVA_LANGUAGE       string = "java"
	AppDJavaAttachImage        = "docker.io/appdynamics/java-agent:latest"
	AGENT_MOUNT_PATH           = "/opt/appdynamics"
	Deployment                 = "Deployment"
	JAVA_TOOL_OPTIONS          = "JAVA_TOOL_OPTIONS"
	FIRST                      = "first"
)

// Add creates a new InstrumentationAgent Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileInstrumentation{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("instrumentation-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource InstrumentationAgent
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.InstrumentationAgent{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner InstrumentationAgent
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.InstrumentationAgent{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileInstrumentation{}

// ReconcileInstrumentation reconciles a InstrumentationAgent object
type ReconcileInstrumentation struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileInstrumentation) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Instrumentation...")

	// Fetch the InstrumentationAgent instance
	instrumentationAgent := &appdynamicsv1alpha1.InstrumentationAgent{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instrumentationAgent)
	reqLogger.Info("Retrieved instrumentation agent.", "Image", instrumentationAgent.Spec.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			reqLogger.Info("Instrumentation Agent resource not found. The object must be deleted")
			r.cleanUp(nil)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Instrumentation Agent")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Instrumentation agent spec exists. Checking the corresponding deployment...")
	// Check if the agent already exists in the namespace
	existingDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instrumentationAgent.Name, Namespace: instrumentationAgent.Namespace}, existingDeployment)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Removing the old instance of the configMap...")
		r.cleanUp(instrumentationAgent)
		reqLogger.Info("Instrumentation agent deployment does not exist. Creating...")
		reqLogger.Info("Checking the secret...")
		secret, esecret := r.ensureSecret(instrumentationAgent)
		if esecret != nil {
			reqLogger.Error(esecret, "Failed to create new Instrumentation Agent due to secret", "Deployment.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
			return reconcile.Result{}, esecret
		}
		reqLogger.Info("Checking the config map")
		econfig := r.ensureConfigMap(instrumentationAgent, secret, true)
		if econfig != nil {
			reqLogger.Error(econfig, "Failed to create new Instrumentation Agent due to config map", "Deployment.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
			return reconcile.Result{}, econfig
		}
		// Define a new deployment for the instrumentation agent
		dep := r.newAgentDeployment(instrumentationAgent, secret)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deployment created successfully. Done")
		r.updateStatus(instrumentationAgent)
		return reconcile.Result{}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Instrumentation agent deployment exists. Checking for deltas with the current state...")
	// Ensure the deployment spec matches the new spec
	// Differentiate between breaking changes and benign updates
	// Check if secret has been recreated. if yes, restart pod
	secret, errsecret := r.ensureSecret(instrumentationAgent)
	if errsecret != nil {
		reqLogger.Error(errsecret, "Failed to get instrumentation agent config secret", "Deployment.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
		return reconcile.Result{}, errsecret
	}

	reqLogger.Info("Retrieving agent config map", "Deployment.Namespace", instrumentationAgent.Namespace)
	econfig := r.ensureConfigMap(instrumentationAgent, secret, false)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain instrumentation agent config map", "Deployment.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
		return reconcile.Result{}, econfig
	}

	breaking, updateDeployment := r.hasBreakingChanges(instrumentationAgent, existingDeployment, secret)

	if breaking {
		fmt.Println("Breaking changes detected. Restarting the instrumentation agent pod...")

		saveOrUpdateInstrumentationAgentSpecAnnotation(instrumentationAgent, existingDeployment)
		errUpdate := r.client.Update(context.TODO(), existingDeployment)
		if errUpdate != nil {
			reqLogger.Error(errUpdate, "Failed to update instrumentation agent", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
			return reconcile.Result{}, errUpdate
		}

		errRestart := r.restartAgent(instrumentationAgent)
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart instrumentation agent", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
			return reconcile.Result{}, errRestart
		}
	} else if updateDeployment {
		fmt.Println("Breaking changes detected. Updating the the instrumentation agent deployment...")
		err = r.client.Update(context.TODO(), existingDeployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update InstrumentationAgent Deployment", "Deployment.Namespace", existingDeployment.Namespace, "Deployment.Name", existingDeployment.Name)
			return reconcile.Result{}, err
		}
	} else {

		reqLogger.Info("No breaking changes.", "instrumentationAgent.Namespace", instrumentationAgent.Namespace)

		statusErr := r.updateStatus(instrumentationAgent)
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

func (r *ReconcileInstrumentation) updateStatus(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {
	instrumentationAgent.Status.LastUpdateTime = metav1.Now()

	if errInstance := r.client.Update(context.TODO(), instrumentationAgent); errInstance != nil {
		return fmt.Errorf("Unable to update InstrumentationAgent instance. %v", errInstance)
	}
	log.Info("InstrumentationAgent instance updated successfully", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "Date", instrumentationAgent.Status.LastUpdateTime)

	err := r.client.Status().Update(context.TODO(), instrumentationAgent)
	if err != nil {
		log.Error(err, "Failed to update instrumentation agent status", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "Deployment.Name", instrumentationAgent.Name)
	} else {
		log.Info("InstrumentationAgent status updated successfully", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "Date", instrumentationAgent.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileInstrumentation) hasBreakingChanges(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent, existingDeployment *appsv1.Deployment, secret *corev1.Secret) (bool, bool) {
	breakingChanges := false
	updateDeployment := false

	fmt.Println("Checking for breaking changes...")

	if existingDeployment.Annotations != nil {
		if oldJson, ok := existingDeployment.Annotations[OLD_SPEC]; ok && oldJson != "" {
			var oldSpec appdynamicsv1alpha1.InstrumentationAgent
			errJson := json.Unmarshal([]byte(oldJson), &oldSpec)
			if errJson != nil {
				log.Error(errJson, "Unable to retrieve the old spec from annotations", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "instrumentationAgent.Name", instrumentationAgent.Name)
			}

			if oldSpec.Spec.ControllerUrl != instrumentationAgent.Spec.ControllerUrl || oldSpec.Spec.Account != instrumentationAgent.Spec.Account {
				breakingChanges = true
			}
		}
	}

	if instrumentationAgent.Spec.Image != "" && existingDeployment.Spec.Template.Spec.Containers[0].Image != instrumentationAgent.Spec.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDeployment.Spec.Template.Spec.Containers[0].Image, instrumentationAgent.Spec.Image)
		existingDeployment.Spec.Template.Spec.Containers[0].Image = instrumentationAgent.Spec.Image
		return false, true
	}

	return breakingChanges, updateDeployment
}

func (r *ReconcileInstrumentation) ensureSecret(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	secretName := AGENT_SECRET_NAME
	if instrumentationAgent.Spec.AccessSecret != "" {
		secretName = instrumentationAgent.Spec.AccessSecret
	}

	key := client.ObjectKey{Namespace: instrumentationAgent.Namespace, Name: secretName}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Required secret %s not found. An empty secret will be created, but the InstrumentationAgent will not start until at least the 'controller-key' key of the secret has a valid value", secretName)

		secret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: instrumentationAgent.Namespace,
			},
		}

		secret.StringData = make(map[string]string)
		secret.StringData["controller-key"] = ""

		errCreate := r.client.Create(context.TODO(), secret)
		if errCreate != nil {
			fmt.Printf("Unable to create secret. %v\n", errCreate)
			return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", errCreate)
		} else {
			fmt.Printf("Secret created. %s\n", secretName)
			errLoad := r.client.Get(context.TODO(), key, secret)
			if errLoad != nil {
				fmt.Printf("Unable to reload secret. %v\n", errLoad)
				return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
			}
		}
	} else if err != nil {
		return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
	}

	return secret, nil
}

func (r *ReconcileInstrumentation) cleanUp(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) {
	namespace := "appdynamics"
	if instrumentationAgent != nil {
		namespace = instrumentationAgent.Namespace
	}
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster-agent-config", Namespace: namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("The old instance of the configMap does not exist")
		return
	}

	if err == nil && cm != nil {
		err = r.client.Delete(context.TODO(), cm)
		if err != nil {
			log.Info("Unable to delete the old instance of the configMap", err)
		} else {
			log.Info("The old instance of the configMap deleted")
		}
	} else {
		log.Error(err, "Unable to retrieve the old instance of the configmap")
	}
}

func (r *ReconcileInstrumentation) ensureConfigMap(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent, secret *corev1.Secret, create bool) error {
	setClusterAgentConfigDefaults(instrumentationAgent)
	setInstrumentationAgentDefaults(instrumentationAgent)

	err := r.ensureAgentMonConfig(instrumentationAgent)
	if err != nil {
		return err
	}

	err = r.ensureAgentConfig(instrumentationAgent)
	if err != nil {
		return err
	}

	err = r.ensureInstrumentationConfig(instrumentationAgent)
	if err != nil {
		return err
	}

	err = r.ensureLogConfig(instrumentationAgent)

	return err

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

func (r *ReconcileInstrumentation) ensureAgentConfig(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_CONFIG_NAME, Namespace: instrumentationAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	errVal, controllerDns, port, sslEnabled := validateControllerUrl(instrumentationAgent.Spec.ControllerUrl)
	if errVal != nil {
		return errVal
	}

	portVal := strconv.Itoa(int(port))

	cm.Name = AGENT_CONFIG_NAME
	cm.Namespace = instrumentationAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] = instrumentationAgent.Spec.Account
	cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] = controllerDns
	cm.Data["APPDYNAMICS_CONTROLLER_PORT"] = portVal
	cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] = sslEnabled
	cm.Data["APPDYNAMICS_CLUSTER_NAME"] = instrumentationAgent.Spec.AppName

	cm.Data["APPDYNAMICS_AGENT_PROXY_URL"] = instrumentationAgent.Spec.ProxyUrl
	cm.Data["APPDYNAMICS_AGENT_PROXY_USER"] = instrumentationAgent.Spec.ProxyUser

	cm.Data["APPDYNAMICS_CLUSTER_MONITORED_NAMESPACES"] = strings.Join(instrumentationAgent.Spec.NsToMonitor, ",")
	cm.Data["APPDYNAMICS_CLUSTER_EVENT_UPLOAD_INTERVAL"] = strconv.Itoa(instrumentationAgent.Spec.EventUploadInterval)
	cm.Data["APPDYNAMICS_CLUSTER_CONTAINER_REGISTRATION_INTERVAL"] = strconv.Itoa(instrumentationAgent.Spec.ContainerRegistrationInterval)
	cm.Data["APPDYNAMICS_CLUSTER_HTTP_CLIENT_TIMEOUT_INTERVAL"] = strconv.Itoa(instrumentationAgent.Spec.HttpClientTimeout)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileInstrumentation) ensureAgentMonConfig(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {
	yml := fmt.Sprintf(`metric-collection-interval-seconds: %d
cluster-metric-collection-interval-seconds: %d
metadata-collection-interval-seconds: %d
container-registration-batch-size: %d
pod-registration-batch-size: %d
metric-upload-retry-count: %d
metric-upload-retry-interval-milliseconds: %d
max-pods-to-register-count: %d
pod-filter: %s`, instrumentationAgent.Spec.MetricsSyncInterval, instrumentationAgent.Spec.ClusterMetricsSyncInterval, instrumentationAgent.Spec.MetadataSyncInterval,
		instrumentationAgent.Spec.ContainerBatchSize, instrumentationAgent.Spec.PodBatchSize,
		instrumentationAgent.Spec.MetricUploadRetryCount, instrumentationAgent.Spec.MetricUploadRetryIntervalMilliSeconds,
		instrumentationAgent.Spec.MaxPodsToRegisterCount,
		createPodFilterString(instrumentationAgent))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_MON_CONFIG_NAME, Namespace: instrumentationAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	cm.Name = AGENT_MON_CONFIG_NAME
	cm.Namespace = instrumentationAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["agent-monitoring.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileInstrumentation) ensureInstrumentationConfig(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {
	yml := fmt.Sprintf(`instrumentation-method: %s
default-match-string: %s
default-label-match: %s
image-info: %v
default-language: %s
ns-to-instrument: %s
resources-to-instrument: %s
default-env: %s
default-custom-agent-config: %s
default-app-name: %s
instrument-container: %s
container-match-string: %s
netviz-info: %v
ns-rules: %v`, instrumentationAgent.Spec.InstrumentationMethod, instrumentationAgent.Spec.DefaultInstrumentMatchString,
		createLabelMapString(instrumentationAgent.Spec.DefaultLabelMatch), createImageInfoString(instrumentationAgent.Spec.ImageInfoMap), instrumentationAgent.Spec.DefaultInstrumentationTech,
		instrumentationAgent.Spec.NsToInstrumentRegex, strings.Join(instrumentationAgent.Spec.ResourcesToInstrument, ","), instrumentationAgent.Spec.DefaultEnv,
		instrumentationAgent.Spec.DefaultCustomConfig, instrumentationAgent.Spec.DefaultAppName, instrumentationAgent.Spec.InstrumentContainer,
		instrumentationAgent.Spec.DefaultContainerMatchString, createNetvizInfoString(instrumentationAgent.Spec.NetvizInfo), createNsRuleString(instrumentationAgent.Spec.NSRules))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: INSTRUMENTATION_CONFIG_NAME, Namespace: instrumentationAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	cm.Name = INSTRUMENTATION_CONFIG_NAME
	cm.Namespace = instrumentationAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["instrumentation.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileInstrumentation) ensureLogConfig(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {
	yml := fmt.Sprintf(`log-level: %s
max-filesize-mb: %d
max-backups: %d
write-to-stdout: %s`, instrumentationAgent.Spec.LogLevel, instrumentationAgent.Spec.LogFileSizeMb, instrumentationAgent.Spec.LogFileBackups,
		strings.ToLower(instrumentationAgent.Spec.StdoutLogging))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_LOG_CONFIG_NAME, Namespace: instrumentationAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent Log configMap. %v", err)
	}

	cm.Name = AGENT_LOG_CONFIG_NAME
	cm.Namespace = instrumentationAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["logger-config.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent Log configMap. %v", e)
		}
		fmt.Println("Log Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent Log configMap. %v", e)
		}
		fmt.Println("Log Configmap updated")
	}

	return nil
}

func (r *ReconcileInstrumentation) ensureSSLConfig(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {

	if instrumentationAgent.Spec.AgentSSLStoreName == "" {
		return nil
	}

	//verify that AGENT_SSL_CRED_STORE_NAME config map exists.
	//it will be copied into the respecive namespace for instrumentation

	existing := &corev1.ConfigMap{}
	errCheck := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_SSL_CRED_STORE_NAME, Namespace: instrumentationAgent.Namespace}, existing)

	if errCheck != nil && errors.IsNotFound(errCheck) {
		return fmt.Errorf("Custom SSL store is requested, but the expected configMap %s with the trusted certificate store not found. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, instrumentationAgent.Namespace)
	} else if errCheck != nil {
		return fmt.Errorf("Unable to validate the expected configMap %s with the trusted certificate store. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, instrumentationAgent.Namespace)
	}

	return nil
}

func (r *ReconcileInstrumentation) newAgentDeployment(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent, secret *corev1.Secret) *appsv1.Deployment {
	if instrumentationAgent.Spec.Image == "" {
		instrumentationAgent.Spec.Image = "appdynamics/cluster-agent:latest"
	}

	if instrumentationAgent.Spec.ServiceAccountName == "" {
		instrumentationAgent.Spec.ServiceAccountName = "appdynamics-operator"
	}

	secretName := AGENT_SECRET_NAME
	if instrumentationAgent.Spec.AccessSecret != "" {
		secretName = instrumentationAgent.Spec.AccessSecret
	}

	fmt.Printf("Building deployment spec for image %s\n", instrumentationAgent.Spec.Image)
	ls := labelsForInstrumentationAgent(instrumentationAgent)
	var replicas int32 = 1
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instrumentationAgent.Name,
			Namespace: instrumentationAgent.Namespace,
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
					ServiceAccountName: instrumentationAgent.Spec.ServiceAccountName,
					Containers: []corev1.Container{{
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AGENT_CONFIG_NAME}}}},
						Env: []corev1.EnvVar{
							{
								Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
										Key:                  "controller-key",
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
						Image:           instrumentationAgent.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "cluster-agent",
						Resources:       instrumentationAgent.Spec.Resources,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "agent-mon-config",
							MountPath: "/opt/appdynamics/cluster-agent/config/agent-monitoring/",
						},
							{
								Name:      "agent-log",
								MountPath: "/opt/appdynamics/cluster-agent/config/logging/",
							},
							{
								Name:      "instrumentation-config",
								MountPath: "/opt/appdynamics/cluster-agent/config/instrumentation",
							}},
					}},
					Volumes: []corev1.Volume{{
						Name: "agent-mon-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AGENT_MON_CONFIG_NAME,
								},
							},
						},
					},
						{
							Name: "instrumentation-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: INSTRUMENTATION_CONFIG_NAME,
									},
								},
							},
						},
						{
							Name: "agent-log",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: AGENT_LOG_CONFIG_NAME,
									},
								},
							},
						}},
				},
			},
		},
	}

	//mount custom SSL cert if necessary (can contain controller and proxy SSL certs)
	if instrumentationAgent.Spec.CustomSSLSecret != "" {
		sslVol := corev1.Volume{
			Name: "agent-ssl-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instrumentationAgent.Spec.CustomSSLSecret,
				},
			},
		}
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, sslVol)

		sslMount := corev1.VolumeMount{
			Name:      "agent-ssl-cert",
			MountPath: "/opt/appdynamics/ssl",
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, sslMount)
	}

	//security context override
	var podSec *corev1.PodSecurityContext = nil
	if instrumentationAgent.Spec.RunAsUser > 0 {
		podSec = &corev1.PodSecurityContext{RunAsUser: &instrumentationAgent.Spec.RunAsUser}
		if instrumentationAgent.Spec.RunAsGroup > 0 {
			podSec.RunAsGroup = &instrumentationAgent.Spec.RunAsGroup
		}
		if instrumentationAgent.Spec.FSGroup > 0 {
			podSec.FSGroup = &instrumentationAgent.Spec.FSGroup
		}
	}

	if podSec != nil {
		dep.Spec.Template.Spec.SecurityContext = podSec
	}

	//image pull secret
	if instrumentationAgent.Spec.ImagePullSecret != "" {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: instrumentationAgent.Spec.ImagePullSecret},
		}
	}

	if instrumentationAgent.Spec.ProxyUser != "" {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: instrumentationAgent.Namespace, Name: AGENT_PROXY_SECRET_NAME}
		err := r.client.Get(context.TODO(), key, secret)
		if err == nil {
			proxyPassword := corev1.EnvVar{
				Name: "APPDYNAMICS_AGENT_PROXY_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_PROXY_SECRET_NAME},
						Key:                  "proxy-password",
					},
				},
			}
			dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, proxyPassword)
		} else {
			log.Error(err, "Unable to load secret cluster-agent-proxy-secret for proxy password.")
		}
	}

	//save the new spec in annotations
	saveOrUpdateInstrumentationAgentSpecAnnotation(instrumentationAgent, dep)

	// Set Instrumentation Agent instance as the owner and controller
	controllerutil.SetControllerReference(instrumentationAgent, dep, r.scheme)
	return dep
}

func saveOrUpdateInstrumentationAgentSpecAnnotation(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent, dep *appsv1.Deployment) {
	jsonObj, e := json.Marshal(instrumentationAgent)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "instrumentationAgent.Namespace", instrumentationAgent.Namespace, "instrumentationAgent.Name", instrumentationAgent.Name)
	} else {
		if dep.Annotations == nil {
			dep.Annotations = make(map[string]string)
		}
		dep.Annotations[OLD_SPEC] = string(jsonObj)
	}
}

func (r *ReconcileInstrumentation) restartAgent(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) error {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForInstrumentationAgent(instrumentationAgent))
	listOps := &client.ListOptions{
		Namespace:     instrumentationAgent.Namespace,
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

func labelsForInstrumentationAgent(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) map[string]string {
	return map[string]string{"name": "instrumentationAgent", "InstrumentationAgent_cr": instrumentationAgent.Name}
}

func setInstrumentationAgentDefaults(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) {
	if instrumentationAgent.Spec.InstrumentationMethod == "" {
		instrumentationAgent.Spec.InstrumentationMethod = ENV_INSTRUMENTATION
	}

	if instrumentationAgent.Spec.DefaultLabelMatch == nil {
		instrumentationAgent.Spec.DefaultLabelMatch = make(map[string]string)
	}

	if instrumentationAgent.Spec.ImageInfoMap == nil {
		instrumentationAgent.Spec.ImageInfoMap = map[string]appdynamicsv1alpha1.ImageInfo{
			JAVA_LANGUAGE: {
				Image:          AppDJavaAttachImage,
				AgentMountPath: AGENT_MOUNT_PATH,
			},
		}
	}

	if instrumentationAgent.Spec.DefaultInstrumentationTech == "" {
		instrumentationAgent.Spec.DefaultInstrumentationTech = JAVA_LANGUAGE
	}

	if instrumentationAgent.Spec.ResourcesToInstrument == nil {
		instrumentationAgent.Spec.ResourcesToInstrument = []string{Deployment}
	}

	if instrumentationAgent.Spec.DefaultEnv == "" {
		instrumentationAgent.Spec.DefaultEnv = JAVA_TOOL_OPTIONS
	}

	if instrumentationAgent.Spec.InstrumentContainer == "" {
		instrumentationAgent.Spec.InstrumentContainer = FIRST
	}

	if instrumentationAgent.Spec.NetvizInfo == (appdynamicsv1alpha1.NetvizInfo{}) {
		instrumentationAgent.Spec.NetvizInfo = appdynamicsv1alpha1.NetvizInfo{
			BciEnabled: true,
			Port:       3892,
		}
	}

	setNsRuleDefault(instrumentationAgent)
}

func setNsRuleDefault(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) {
	for i := range instrumentationAgent.Spec.NSRules {
		if instrumentationAgent.Spec.NSRules[i].EnvToUse == "" {
			instrumentationAgent.Spec.NSRules[i].EnvToUse = instrumentationAgent.Spec.DefaultEnv
		}

		if instrumentationAgent.Spec.NSRules[i].LabelMatch == nil {
			instrumentationAgent.Spec.NSRules[i].LabelMatch = make(map[string]string)
		}

		if instrumentationAgent.Spec.NSRules[i].Language == "" {
			instrumentationAgent.Spec.NSRules[i].Language = instrumentationAgent.Spec.DefaultInstrumentationTech
		}

		if instrumentationAgent.Spec.NSRules[i].InstrumentContainer == "" {
			instrumentationAgent.Spec.NSRules[i].InstrumentContainer = instrumentationAgent.Spec.InstrumentContainer
		}

		if instrumentationAgent.Spec.NSRules[i].ContainerNameMatchString == "" {
			instrumentationAgent.Spec.NSRules[i].ContainerNameMatchString = instrumentationAgent.Spec.DefaultContainerMatchString
		}

		if instrumentationAgent.Spec.NSRules[i].AppName == "" {
			instrumentationAgent.Spec.NSRules[i].AppName = instrumentationAgent.Spec.DefaultAppName
		}

		if instrumentationAgent.Spec.NSRules[i].CustomAgentConfig == "" {
			instrumentationAgent.Spec.NSRules[i].CustomAgentConfig = instrumentationAgent.Spec.DefaultCustomConfig
		}
	}
}

func setClusterAgentConfigDefaults(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) {
	// bootstrap-config defaults
	if instrumentationAgent.Spec.NsToMonitor == nil {
		instrumentationAgent.Spec.NsToMonitor = []string{"default"}
	}

	if instrumentationAgent.Spec.EventUploadInterval == 0 {
		instrumentationAgent.Spec.EventUploadInterval = 10
	}

	if instrumentationAgent.Spec.ContainerRegistrationInterval == 0 {
		instrumentationAgent.Spec.ContainerRegistrationInterval = 120
	}

	if instrumentationAgent.Spec.HttpClientTimeout == 0 {
		instrumentationAgent.Spec.HttpClientTimeout = 30
	}

	// agent-monitoring defaults
	if instrumentationAgent.Spec.MetricsSyncInterval == 0 {
		instrumentationAgent.Spec.MetricsSyncInterval = 30
	}

	if instrumentationAgent.Spec.ClusterMetricsSyncInterval == 0 {
		instrumentationAgent.Spec.ClusterMetricsSyncInterval = 60
	}

	if instrumentationAgent.Spec.MetadataSyncInterval == 0 {
		instrumentationAgent.Spec.MetadataSyncInterval = 60
	}

	if instrumentationAgent.Spec.ContainerBatchSize == 0 {
		instrumentationAgent.Spec.ContainerBatchSize = 5
	}

	if instrumentationAgent.Spec.PodBatchSize == 0 {
		instrumentationAgent.Spec.PodBatchSize = 6
	}

	if instrumentationAgent.Spec.MetricUploadRetryCount == 0 {
		instrumentationAgent.Spec.MetricUploadRetryCount = 2
	}

	if instrumentationAgent.Spec.MetricUploadRetryIntervalMilliSeconds == 0 {
		instrumentationAgent.Spec.MetricUploadRetryIntervalMilliSeconds = 5
	}

	if instrumentationAgent.Spec.MaxPodsToRegisterCount == 0 {
		instrumentationAgent.Spec.MaxPodsToRegisterCount = 750
	}

	// logger defaults
	if instrumentationAgent.Spec.LogLevel == "" {
		instrumentationAgent.Spec.LogLevel = "INFO"
	}

	if instrumentationAgent.Spec.LogFileSizeMb == 0 {
		instrumentationAgent.Spec.LogFileSizeMb = 5
	}

	if instrumentationAgent.Spec.LogFileBackups == 0 {
		instrumentationAgent.Spec.LogFileBackups = 3
	}

	if instrumentationAgent.Spec.StdoutLogging == "" {
		instrumentationAgent.Spec.StdoutLogging = "true"
	}
}

func parseLabelField(labels []map[string]string, labelsType string) string {
	var stringBuilder strings.Builder
	stringBuilder.WriteString(" " + labelsType + "-labels: [")
	for _, label := range labels {
		for labelKey, labelValue := range label {
			stringBuilder.WriteString(fmt.Sprintf("{ %s: %s },", labelKey, labelValue))
		}
	}
	labelsString := strings.TrimRight(stringBuilder.String(), ",")
	return labelsString
}

func parseNameField(names []string, namesType string) string {
	var stringBuilder strings.Builder
	stringBuilder.WriteString(" " + namesType + "-names: [")
	for _, name := range names {
		stringBuilder.WriteString(name + ",")
	}
	namesString := strings.TrimRight(stringBuilder.String(), ",")
	return namesString
}

func createPodFilterString(instrumentationAgent *appdynamicsv1alpha1.InstrumentationAgent) string {
	var podFilterString strings.Builder
	podFilterString.WriteString("{")
	if instrumentationAgent.Spec.PodFilter.BlacklistedLabels != nil {
		podFilterString.
			WriteString(parseLabelField(instrumentationAgent.Spec.PodFilter.BlacklistedLabels, BLACKLISTED) + "],")
	}

	if instrumentationAgent.Spec.PodFilter.WhitelistedLabels != nil {
		podFilterString.
			WriteString(parseLabelField(instrumentationAgent.Spec.PodFilter.WhitelistedLabels, WHITELISTED) + "],")
	}

	if instrumentationAgent.Spec.PodFilter.BlacklistedNames != nil {
		podFilterString.
			WriteString(parseNameField(instrumentationAgent.Spec.PodFilter.BlacklistedNames, BLACKLISTED) + "],")
	}

	if instrumentationAgent.Spec.PodFilter.WhitelistedNames != nil {
		podFilterString.
			WriteString(parseNameField(instrumentationAgent.Spec.PodFilter.WhitelistedNames, WHITELISTED) + "],")
	}
	return strings.TrimRight(podFilterString.String(), ",") + "}"
}

func createNetvizInfoString(netvizInfo appdynamicsv1alpha1.NetvizInfo) string {
	netvizInfoMap := map[string]interface{}{
		"bci-enabled": netvizInfo.BciEnabled,
		"port":        netvizInfo.Port,
	}
	json, err := json.Marshal(netvizInfoMap)
	if err != nil {
		return ""
	}
	return string(json)
}

func createImageInfoString(imageInfo map[string]appdynamicsv1alpha1.ImageInfo) string {
	imageInfoMap := make(map[string]map[string]string)
	for language, info := range imageInfo {
		imageInfo := map[string]string{
			"image":            info.Image,
			"agent-mount-path": info.AgentMountPath,
		}
		imageInfoMap[strings.ToLower(language)] = imageInfo
	}
	json, err := json.Marshal(imageInfoMap)
	if err != nil {
		return ""
	}
	return string(json)
}

func createLabelMapString(m map[string]string) string {
	json, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(json)
}

func createNsRuleString(rules []appdynamicsv1alpha1.NSRule) string {
	rulesOut := make([]map[string]interface{}, 0)
	for _, rule := range rules {
		ruleMap := map[string]interface{}{
			"namespaces":             rule.NamespaceRegex,
			"match-string":           rule.MatchString,
			"label-match":            rule.LabelMatch,
			"app-name":               rule.AppName,
			"tier-name":              rule.TierName,
			"instrument-container":   rule.InstrumentContainer,
			"container-match-string": rule.ContainerNameMatchString,
			"custom-agent-config":    rule.CustomAgentConfig,
			"env":                    rule.EnvToUse,
		}
		rulesOut = append(rulesOut, ruleMap)
	}
	json, err := json.Marshal(rulesOut)
	if err != nil {
		return ""
	}
	return string(json)
}
