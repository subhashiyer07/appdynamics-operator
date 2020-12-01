package clustercollector

import (
	"context"
	"encoding/json"
	"fmt"
	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clusterCollectorController struct {
	client           client.Client
	clusterCollector *appdynamicsv1alpha1.Clustercollector
	deployment       *appsv1.Deployment
}

func NewClusterCollectorController(client client.Client, clusterCollector *appdynamicsv1alpha1.Clustercollector) *clusterCollectorController {
	clusterCollectorCtrl := &clusterCollectorController{
		client:           client,
		clusterCollector: clusterCollector,
		deployment:       &appsv1.Deployment{},
	}
	return clusterCollectorCtrl
}

func (c *clusterCollectorController) Init(reqLogger logr.Logger) (bool, error) {
	err := c.initialiseDeployment()
	newDeploymentCreated := false
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Cluster Collector deployment does not exist. Creating...")
		newDeploymentCreated = true
		// Define a new deployment for the cluster collector
		err = c.newCollectorDeployment()
		return newDeploymentCreated, err
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return newDeploymentCreated, err
	}
	return newDeploymentCreated, err
}

func (c *clusterCollectorController) Create(reqLogger logr.Logger) error {
	reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", c.deployment.Namespace, "Deployment.Name", c.deployment.Name)
	err := c.client.Create(context.TODO(), c.deployment)
	if err != nil {
		reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", c.deployment.Namespace, "Deployment.Name", c.deployment.Name)
		return err
	}
	reqLogger.Info("Deployment created successfully. Done")
	return nil
}

func (c *clusterCollectorController) Update(reqLogger logr.Logger) (bool, error) {
	breaking, updateDeployment := hasBreakingChanges(c.clusterCollector, c.deployment)

	existingDeployment := c.deployment
	clusterCollector := c.clusterCollector
	if breaking {
		fmt.Println("Breaking changes detected. Restarting the cluster collector pod...")

		c.saveOrUpdateClusterCollectorSpecAnnotation()

		errUpdate := c.client.Update(context.TODO(), existingDeployment)
		if errUpdate != nil {
			reqLogger.Error(errUpdate, "Failed to update cluster collector", "clusterCollector.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
			return false, errUpdate
		}

		errRestart := c.RestartCollector()
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart cluster collector", "clusterCollector.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
			return false, errRestart
		}
	} else if updateDeployment {
		fmt.Println("Breaking changes detected. Updating the the cluster collector deployment...")
		err := c.client.Update(context.TODO(), existingDeployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update Clustercollector Deployment", "Deployment.Namespace", existingDeployment.Namespace, "Deployment.Name", existingDeployment.Name)
			return false, err
		}
	} else {
		reqLogger.Info("No breaking changes.", "clusterCollector.Namespace", clusterCollector.Namespace)
		statusErr := updateStatus(clusterCollector, c.client)
		if statusErr == nil {
			reqLogger.Info("Status updated. Exiting reconciliation loop.")
		} else {
			reqLogger.Info("Status not updated. Exiting reconciliation loop.")
		}
	}
	return true, nil
}

func (c *clusterCollectorController) RestartCollector() error {
	clusterCollector := c.clusterCollector
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForClusterCollector(clusterCollector))
	listOps := &client.ListOptions{
		Namespace:     clusterCollector.Namespace,
		LabelSelector: labelSelector,
	}
	err := c.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve cluster-collector pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = c.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete cluster-collector pod. %v", err)
	}
	return nil
}

func (c *clusterCollectorController) initialiseDeployment() error {
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.clusterCollector.Name,
		Namespace: c.clusterCollector.Namespace}, c.deployment)
	return err
}

func (c *clusterCollectorController) saveOrUpdateClusterCollectorSpecAnnotation() {
	jsonObj, e := json.Marshal(c.clusterCollector)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "clusterCollector.Namespace", c.clusterCollector.Namespace, "clusterCollector.Name", c.clusterCollector.Name)
	} else {
		if c.deployment.Annotations == nil {
			c.deployment.Annotations = make(map[string]string)
		}
		c.deployment.Annotations[OLD_SPEC] = string(jsonObj)
	}
}

func (c *clusterCollectorController) newCollectorDeployment() error {
	fmt.Printf("Building deployment spec for image %s\n", c.clusterCollector.Spec.Image)
	ls := labelsForClusterCollector(c.clusterCollector)
	var replicas int32 = 1
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.clusterCollector.Name,
			Namespace: c.clusterCollector.Namespace,
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
					ServiceAccountName: c.clusterCollector.Spec.ServiceAccountName,
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
						Image:           c.clusterCollector.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "cluster-collector",
						Resources:       c.clusterCollector.Spec.Resources,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "clustermon-config",
								MountPath: "/opt/appdynamics/InfraAgent/collectors/clustermon.conf",
								SubPath:   "clustermon.conf",
							},
							{
								Name:      "infraagent-config",
								MountPath: "/opt/appdynamics/InfraAgent/agent.conf",
								SubPath:   "agent.conf",
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
	c.deployment = dep
	c.saveOrUpdateClusterCollectorSpecAnnotation()
	return nil
}

func labelsForClusterCollector(clusterCollector *appdynamicsv1alpha1.Clustercollector) map[string]string {
	return map[string]string{"name": "clusterCollector", "clusterCollector_cr": clusterCollector.Name}
}

func (c *clusterCollectorController) GetDeployment() *appsv1.Deployment {
	return c.deployment
}
