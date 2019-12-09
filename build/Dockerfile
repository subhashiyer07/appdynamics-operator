FROM alpine:3.8

LABEL name="AppDynamics ClusterAgent Operator" \
      vendor="AppDynamics" \
      version="0.3.6" \
      release="1" \
      url="https://www.appdynamics.com" \
      summary="AppDynamics monitoring solution for applications deployed to Kubernetes clusters" \
      description="The ClusterAgent monitors state of Kuberenetes resources and derives metrics to provide visibility into common application impacting issues."


RUN apk upgrade --update --no-cache

USER nobody

ADD build/_output/bin/appdynamics-operator /usr/local/bin/appdynamics-operator
ADD LICENSE /licenses/