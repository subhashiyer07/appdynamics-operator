# AppDynamics ClusterAgent Operator

AppDynamics ClusterAgent Operator simplifies the configuration and lifecycle management of the AppDynamics ClusterAgent on Kubernetes and OpenShift. The Operator encapsulates key operational knowledge on how to configure and upgrade the ClusterAgent. It knows, for example, which configuration changes are benign and do not require restart of the ClusterAgent, which minimizes unnecesary API calls and the load on the cluster.
The Operator is implemented using OperatorSDK and uses Kubernetes API to maintain the desired state of the ClusterAgent. The Operator works with the ClusterAgent as a custom resource with its own definition of properties (CRD). This level of abstraction further simplifies the management of monitoring and imstrumentation and ensures granular security policy of the ClusterAgent deployment.

## Operator deployment
Create namespace for the operator and the ClusterAgent
`kubectl create namespace appdynamics`

Create secret. APPDYNAMICS_REST_API_CREDENTIALS is required
`kubectl -n appdynamics create secret generic appd-clusteragent --from-literal=APPDYNAMICS_REST_API_CREDENTIALS=<username>@<account>:<password> --from-literal=APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY=<controller access key> --from-literal=APPDYNAMICS_EVENT_ACCESS_KEY=<events api key> 

Deploy

### Kubernetes

### OpenShift





## Deployimg the ClusterAgent

An example spec. (quick start)
apiVersion: appdynamics.com/v1alpha1
kind: ClusterAgent
metadata:
  name: local-k8s
spec:
  controllerUrl: "http://455controllernossh-k8sbiqtest-eq2w7bwd.srv.ravcloud.com:8090"
  appDJavaAttachImage: "appdynamics/java-agent-attach:4.5.5"
  appDDotNetAttachImage: "appdynamics/dotnet-agent-agent:1.1"
  nsToInstrument:
    - dev
	- ad-devops
  instrumentRule:
	- matchString: "client-api"
	  namespaces
	  appDAppLabel: "appName"
	  #appDTierLabel: "tierName"
	  #version: "appdynamics/java-agent:4.5.6"
	  #tech: "java"
	  #method: "mountenv"
      #biq: "sidecar"
	
### Required settings

### Optional settings

