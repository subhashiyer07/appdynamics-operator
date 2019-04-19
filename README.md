# AppDynamics ClusterAgent Operator

AppDynamics ClusterAgent Operator simplifies the configuration and lifecycle management of the AppDynamics ClusterAgent on Kubernetes and OpenShift. The Operator encapsulates key operational knowledge on how to configure and upgrade the ClusterAgent. It knows, for example, which configuration changes are benign and do not require restart of the ClusterAgent, which minimizes unnecesary load on the cluster API server.
The Operator is implemented using OperatorSDK and uses Kubernetes API to maintain the desired state of the ClusterAgent. The Operator works with the ClusterAgent as a custom resource with its own definition of properties (CRD). This level of abstraction further simplifies the management of monitoring and imstrumentation and ensures granular security policy of the ClusterAgent deployment.



## Operator deployment
Create namespace for the operator and the ClusterAgent
`kubectl create namespace appdynamics`

Create secret. APPDYNAMICS_REST_API_CREDENTIALS is required
`kubectl -n appdynamics create secret generic appd-clusteragent --from-literal=APPDYNAMICS_REST_API_CREDENTIALS=<username>@<account>:<password> --from-literal=APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY=<controller access key> --from-literal=APPDYNAMICS_EVENT_ACCESS_KEY=<events api key> 

* Create namespace for AppDynamics components
  * Kubernetes
   `kubectl create namespace appdynamics-infra`
  * OpenShift
   `oc new-project appdynamics-infra --description="AppDynamics Infrastructure"`

* Create Secret `cluster-agent-secret` (deploy/cluster-agent/cluster-agent-secret.yaml). 
  * The "api-user" key with the AppDynamics user account information is required. It needs to be in the following format <username>@<account>:<password>, e.g ` user@customer1:123 `. 
  * The other 2 keys, "controller-key" and "event-key", are optional. If not specified, they will be automatically created by the ClusterAgent

`
kubectl -n appdynamics-infra create secret generic cluster-agent-secret \
--from-literal=api-user="" \
--from-literal=controller-key="" \
--from-literal=event-key="" \
`

* Update the image reference in the Operator deployment spec (deploy/operator.yaml). The default is "docker.io/appdynamics/cluster-agent-operator:latest".


* Deploy the ClusterAgent
 `kubectl create -f deploy/`


## ClusterAgent deployment

Here is an example of a minimalistic spec of the ClusterAgent custom resource:

```
apiVersion: appdynamics.com/v1alpha1
kind: ClusterAgent
metadata:
  name: K8s-Cluster-Agent
spec:
  controllerUrl: "<protocol>://<controller-url>:<port>"
```
Update controller URL in the configMap (deploy/cluster-agent/cluster-agent-config.yaml). The controller URL must be in the following format:
` <protocol>://<controller-url>:<port> `

Here is a more involved spec with imstrumentation rules

```
apiVersion: appdynamics.com/v1alpha1
kind: Clusteragent
metadata:
  name: local-k8s
spec:
  controllerUrl: ""
  appDJavaAttachImage: "appdynamics/java-agent:latest"
  appDDotNetAttachImage: "appdynamics/dotnet-core-agent:latest"
  nsToInstrument:
    - dev
	- ad-devops
  instrumentRule:
	- matchString: "client-api"
	  namespaces:
	    - prod
	  appDAppLabel: "appName"
	  appDTierLabel: "tierName"
	  version: "appdynamics/java-agent:4.5.6"
	  tech: "java"
	  method: "mountEnv"
      biq: "sidecar"
```
	

