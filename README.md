# AppDynamics Operator

AppDynamics Operator simplifies the configuration and lifecycle management of the AppDynamics ClusterAgent and the AppDynamics Machine Agent on different Kubernetes distributions and OpenShift. The Operator encapsulates key operational knowledge on how to configure and upgrade the ClusterAgent and the Machine Agent. It knows, for example, which configuration changes are benign and do not require restart of the ClusterAgent, which minimizes unnecesary load on the cluster API server.

The Operator is implemented using [OperatorSDK](https://github.com/operator-framework/operator-sdk) and uses Kubernetes API to maintain the desired state of the custom resources that represent the ClusterAgent and the Machine Agent.
When the Operator is deployed, it creates custom resource definitions (CRDs) for 2 custom resources:

* [clusteragent](https://github.com/Appdynamics/appdynamics-operator#clusteragent-deployment), which represents the ClusterAgent.
* [infraviz](https://github.com/Appdynamics/appdynamics-operator#the-machine-agent-deployment), which represents the Machine Agent bundled with netviz and analytics.


This level of abstraction further simplifies the management of monitoring and imstrumentation and ensures granular security policy of the ClusterAgent and the Machine Agent.



## Operator deployment
Create namespace for the operator and the ClusterAgent

* Create namespace for AppDynamics components
  * Kubernetes
   `kubectl create namespace appdynamics`
  * OpenShift
   `oc new-project appdynamics --description="AppDynamics Infrastructure"`

* Create Secret `cluster-agent-secret` with the following key
  * The "controller-key" - the access key to the AppDynamics controller.

```
kubectl -n appdynamics create secret generic cluster-agent-secret \
--from-literal=controller-key="<controller-access-key>" \
```

* Update the image reference in the Operator deployment spec (deploy/cluster-agent-operator.yaml), if necessary.

The default is "docker.io/appdynamics/cluster-agent-operator:latest".


* Deploy the Operator
```
kubectl create -f deploy/cluster-agent-operator.yaml
```
* On OpenShift
```
oc create -f deploy/cluster-agent-operator-openshift.yaml
```


### Images

By default "docker.io/appdynamics/cluster-agent-operator:latest" is used.

[AppDynamics images](https://access.redhat.com/containers/#/product/f5e13e601dc05eaa) are also available from [Red Hat Container Catalog](https://access.redhat.com/containers/).

To enable pulling,  create a secret in the ClusterAgent namespace. In this example, namespace **appdynamics** is used and appdynamics-operator account is linked to the secret.

```
$ oc -n appdynamics create secret docker-registry redhat-connect
--docker-server=registry.connect.redhat.com
--docker-username=REDHAT_CONNECT_USERNAME
--docker-password=REDHAT_CONNECT_PASSWORD --docker-email=unused
$ oc -n appdynamics secrets link appdynamics-operator redhat-connect
--for=pull
```


## ClusterAgent deployment

The AppDynamics Cluster Agent is a new, lightweight agent written in Golang providing APM monitoring for Kubernetes. The Cluster Agent is designed to help you understand how Kubernetes infrastructure affects your applications and business performance. With the AppDynamics Cluster Agent, you can collect metadata, metrics, and events about a Kubernetes cluster. You can query for the components in the cluster, and report metrics for those components. The Cluster Agent works across cloud platforms such as Kubernetes on AWS EKS, AKS on Azure, PKS on Pivotal and many others.
The `clusteragent` is the custom resource that the Operator works with to deploy an instance of the ClusterAgent. When a clusteragent spec is provided, the Operator will create a single replica deployment and the necessary additional resources (a configMap and a service) to support the ClusterAgent.

Here is an example of a minimalistic spec of the ClusterAgent custom resource:

```
apiVersion: appdynamics.com/v1alpha1
kind: Clusteragent
metadata:
  name: k8s-cluster-agent
  namespace: appdynamics
spec:
  appName: "<app-name>"
  controllerUrl: "<protocol>://<appdynamics-controller-host>:<port>"
  account: "<account-name>"
  image: "<your-docker-registry>/appdynamics/cluster-agent:tag"
```

### Clusteragent Configuration Settings


| Parameter                 | Description                                                  | Default                    |
| ------------------------- | ------------------------------------------------------------ | -------------------------- |
| `controllerUrl`           |  Full AppDynamics Controller URL including protocol and port |       Required             |
| `account`                 |  AppDynamics Account Name                                    |       Required             |
| `image` | Cluster Agent image reference | Required |
| `imagePullSecret` | Name of the image pull secret | Optional
| `resources` |  Definitions of resources and limits for the Cluster Agent | See resource recommendations below |
| `appName` |  Name of the cluster. Displayed in the Controller UI as your cluster name. | Required |
| `nsToMonitor` | List of namespaces the Cluster Agent should initially monitor | Default is `default` |
| `eventUploadInterval` | Interval in seconds at which Kubernetes warning and state-change events are uploaded to the Controller | Default 10 sec |
| `containerRegistrationInterval` | Interval in seconds at which the Cluster Agent checks for containers and registers them with the Controller. You should only modify the default value if you want to discover running containers more frequently. The default value should be used in most environments. | Default 120 sec |
| `httpClientTimeout` | Number of seconds after which the server call is terminated if no response is received from the Controller | Default 30 sec |
| `customSSLSecret` | Provides the certificates to the Cluster Agent | Not set by default |
| `proxyUrl` | Publicly accessible hostname of the proxy (`protocol://domain:port`) | Not set by default |
| `proxyUser` | Proxy username associated with the basic authentication credentials | Not set by default |
| `metricsSyncInterval` | Interval in seconds between sending container metrics to the Controller | Default 30 sec |
| `clusterMetricsSyncInterval` | Interval in seconds between sending cluster-level metrics to the Controller | Default 60 sec |
| `metadataSyncInterval` | Interval in seconds at which metadata is collected for containers and pods | Default 60 sec |
| `podFilter` | Definitions of whitelisted/blacklisted names and labels to filter pods | Not set by default |
| `containerBatchSize` |The Cluster Agent checks for containers and registers them with the Controller. This process is known as a container registration cycle. The containers are sent to the Controller in batches, and the containerBatchSize is the maximum number of containers per batch in one cycle. | Default 5 containers |
| `podBatchSize` | The Cluster Agent checks for pods and registers them with the Controller. This process is known as a pod registration cycle. The pods are sent to the Controller in batches, and the podBatchSize is the maximum number of pods per batch in one registration cycle. | Default 6 pods |
| `metricUploadRetryCount` | Number of times metric upload action to be attempted if unsuccessful the first time | Default is 3 |
| `metricUploadRetryIntervalMilliSeconds` | Interval between consecutive metric upload retries, in milliseconds | Default is 5 |
| `logLevel` | Logging level (`INFO`, `DEBUG`, `WARN`, `TRACE`) | Default `INFO` |
| `logFileSizeMb` | Maximum file size of the log in MB | Default is 5 |
| `logFileBackups` | Maximum number of backups the log saves. When the maximum number of backups is reached, the oldest log file after the initial log file is deleted. | Default is 3 |
| `stdoutLogging` | By default, the Cluster Agent writes to a log file in the logs directory. The stdoutLogging parameter is provided so you can send logs to the container stdout as well. | Default is `"true"` |
| `runAsUser` | ID of the user that will be associated with the cluster agent pod. | By default, the agent container runs as user *appdynamics*
| `runAsGroup` | ID of the group that will be associated with the cluster agent pod. | By default, the group is *appdynamics*

Example resource limits:

```
   resources:
    limits:
      cpu: 300m
      memory: "200M"
    requests:
      cpu: 200m
      memory: "100M"
```

Here is an example of the entire spec of the ClusterAgent custom resource:
```
    apiVersion: appdynamics.com/v1alpha1
    kind: Clusteragent
    metadata:
      name: k8s-cluster-agent
      namespace: appdynamics
    spec:
      appName: "<app-name>"
      controllerUrl: "<protocol>://<appdynamics-controller-host>:<port>"
      account: "<account-name>"
      # docker image info
      image: "<your-docker-registry>/appdynamics/cluster-agent:tag"
      nsToMonitor:
        - "default"
      eventUploadInterval: 10
      containerRegistrationInterval: 120
      httpClientTimeout: 30
      customSSLSecret: "<secret-name>"
      proxyUrl: "<protocol>://<domain>:<port>"
      proxyUser: "<proxy-user>"
      metricsSyncInterval: 30
      clusterMetricsSyncInterval: 60
      metadataSyncInterval: 60
      containerBatchSize: 25
      podBatchSize: 30
      metricUploadRetryCount: 3
      metricUploadRetryIntervalMilliSeconds: 5
      podFilter:
        blacklistedLabels:
          - label1: value1
        whitelistedLabels:
          - label1: value1
          - label2: value2
        whitelistedNames:
          - name1
        blacklistedNames:
          - name1
          - name2
      logLevel: "INFO"
      logFileSizeMb: 5
      logFileBackups: 3
      stdoutLogging: "true"
```


## The Machine Agent deployment

Appdynamics operator can be used to enable server and network visibility with AppDynamics Machine agent.
The operator works with custom resource `infraviz` to deploy the AppDynamics Machine Agent daemon set.

Here is an example of a minimalistic `infraviz` spec with the required parameters:

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: "https://appd-controller.com"
  image: "docker.io/appdynamics/machine-agent-analytics:latest"
  account: "<your-account-name>"
  globalAccount: "<your-global-account-name"

```

 The controller URL must be in the following format:
` <protocol>://<controller-domain>:<port> `

### Infraviz Configuration Settings


| Parameter                 | Description                                                  | Default                    |
| ------------------------- | ------------------------------------------------------------ | -------------------------- |
| `controllerUrl`           |  Url of the AppDynamics controller                            |       Required             |
| `account`                 |  AppDynamics Account Name                                    |       Required             |
| `globalAccount`          |  Global Account Name                            |     Required  |
| `eventServiceUrl`   | Event Service Endpoint | Optional |
| `enableContainerHostId` | Flag that determines how container names are derived (pod name vs container id) | "true" |
| `enableServerViz` |  Enable Server Visibility | "true" |
| `enableDockerViz` | Enable Docker Container Visibiltiy | "true" |
| `uniqieHostId` | Unique host ID in AppDynamics. | Optional |
| `metricsLimit` | Number of metrics that the Machine Agent is allowed to post to the controller | Optional |
| `logLevel`	| Logging level (`info` or `debug`) | `info` |
| `stdoutLogging` | Determines if the logs are saved to a file or redirected to the console | "false" |
| `netVizPort` | When > 0, the network visibility agent will be deployed in a sidecar along with the machine agent. By default the network visibility agent works with port `3892` | Not set by default |
| `proxyUrl` | Url of the proxy server (protocol://domain:port") | Optional |
| `proxyUser` | Proxy user credentials (user@password) | Optional |
| `propertyBag` | A string with any other machine agent parameters | Optional
| `pks` | PKS deployment flag. Must be used on PKS environments|  Optional |
| `enableMasters` | When set to **true** server visibility will be provided for Master nodes. By default only Worker nodes are monitored. On managed Kubernetes providers the flag has no effect, as the Master plane is not accessible | Optional
| `image` | The Machine Agent image | "appdynamics/machine-agent-analytics:latest" |
| `nodeSelector` | A set of nodes to deploy the daemon set pods to | Optional |
| `tolerations` | A list of tolerations | Optional |
| `env` | List of environment variables | Optional |
| `args` | List of command arguments | Optional
| `resources` | Definitions of resources and limits for the machine agent  | See example below |

Example resource limits:

```
   resources:
    limits:
      cpu: 600m
      memory: "1G"
    requests:
      cpu: 300m
      memory: "800M"
```
