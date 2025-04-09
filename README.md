# resource-backup

kubectl plugin that backs up Kubernetes objects (including CRDs) to the local file system. Before saving any resource, the plugin does some additional processing to remove:
- the status stanza if the object has any.
- the server generated fields from the object metadata.
- any field with a `null` value.

The plugin aims to make the saved objects look like the original creation request. However, the plugin does not remove the fields that has a default value (unlike the neat [plugin](https://github.com/itaysk/kubectl-neat)) because it's not possible to make a distinction between a value set by a creation/update request and a value set by a controller or a mutating admission webhook. If we take the deployment of an ingress-ngix below as an example, the fields surrounded with ascii boxes will be removed from the saved objects.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
#removed ----------------------------------------------------|
  creationTimestamp: "2023-10-01T15:45:42Z"                 #|
  generation: 1                                             #|
#------------------------------------------------------------|
  labels:
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: ingress-nginx
    app.kubernetes.io/name: ingress-nginx
  name: ingress-nginx-controller
  namespace: ingress-nginx
#removed ----------------------------------------------------|
  resourceVersion: "8039704"                                #|
  uid: 63e16d11-a32e-498a-a8d8-f1b9df9d15a0                 #|
#------------------------------------------------------------|
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app.kubernetes.io/component: controller
      app.kubernetes.io/instance: ingress-nginx
      app.kubernetes.io/name: ingress-nginx
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
#removed ----------------------------------------------------|
      creationTimestamp: null                               #|
#------------------------------------------------------------|
      labels:
        app.kubernetes.io/component: controller
        app.kubernetes.io/instance: ingress-nginx
        app.kubernetes.io/name: ingress-nginx
        gcp-auth-skip-secret: "true"
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
        - --election-id=ingress-nginx-leader
        - --controller-class=k8s.io/ingress-nginx
        - --watch-ingress-without-class=true
        - --configmap=$(POD_NAMESPACE)/ingress-nginx-controller
        - --tcp-services-configmap=$(POD_NAMESPACE)/tcp-services
        - --udp-services-configmap=$(POD_NAMESPACE)/udp-services
        - --validating-webhook=:8443
        - --validating-webhook-certificate=/usr/local/certificates/cert
        - --validating-webhook-key=/usr/local/certificates/key
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: LD_PRELOAD
          value: /usr/local/lib/libmimalloc.so
        image: registry.k8s.io/ingress-nginx/controller:v1.8.1@sha256:e5c4824e7375fcf2a393e1c03c293b69759af37a9ca6abdb91b13d78a93da8bd
        imagePullPolicy: IfNotPresent
        lifecycle:
          preStop:
            exec:
              command:
              - /wait-shutdown
        livenessProbe:
          failureThreshold: 5
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        name: controller
        ports:
        - containerPort: 80
          hostPort: 80
          name: http
          protocol: TCP
        - containerPort: 443
          hostPort: 443
          name: https
          protocol: TCP
        - containerPort: 8443
          name: webhook
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          requests:
            cpu: 100m
            memory: 90Mi
        securityContext:
          allowPrivilegeEscalation: true
          capabilities:
            add:
            - NET_BIND_SERVICE
            drop:
            - ALL
          runAsUser: 101
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /usr/local/certificates/
          name: webhook-cert
          readOnly: true
      dnsPolicy: ClusterFirst
      nodeSelector:
        kubernetes.io/os: linux
        minikube.k8s.io/primary: "true"
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccount: ingress-nginx
      serviceAccountName: ingress-nginx
      terminationGracePeriodSeconds: 0
      tolerations:
      - effect: NoSchedule
        key: node-role.kubernetes.io/master
        operator: Equal
      volumes:
      - name: webhook-cert
        secret:
          defaultMode: 420
          secretName: ingress-nginx-admission
#removed--------------------------------------------------------------------------------------------------|
status:                                                                                                   #|
  availableReplicas: 1                                                                                    #|
  conditions:                                                                                             #|
  - lastTransitionTime: "2023-10-01T15:45:54Z"                                                            #|
    lastUpdateTime: "2023-10-01T15:45:54Z"                                                                #|
    message: Deployment has minimum availability.                                                         #|
    reason: MinimumReplicasAvailable                                                                      #|
    status: "True"                                                                                        #|
    type: Available                                                                                       #|
  - lastTransitionTime: "2023-10-01T15:45:54Z"                                                            #|
    lastUpdateTime: "2023-10-01T15:49:21Z"                                                                #|
    message: ReplicaSet "ingress-nginx-controller-7799c6795f" has successfully progressed.                #|
    reason: NewReplicaSetAvailable                                                                        #|
    status: "True"                                                                                        #|
    type: Progressing                                                                                     #|
  observedGeneration: 1                                                                                   #|
  readyReplicas: 1                                                                                        #|
  replicas: 1                                                                                             #|
  updatedReplicas: 1                                                                                      #|
#---------------------------------------------------------------------------------------------------------|
```
# Installation

  ```sh
  kubectl krew install backup
  ```

# Usage:

```
usage: kubectl backup [<flags>] <resource>


Flags:
      --[no-]help            Show context-sensitive help (also try --help-long and --help-man).
  -n, --namespace="default"  if the resource is namespaced, this flag sets the namespace scope
      --dir="."              the directory where the resources will be saved
      --[no-]version         Show application version.

Args:
  <resource>  the Kubernetes resource to backup. e.g deployment, service,...

```

For example, assuming that the namespace `ns` contains three deployments: `deployment1`, `deployment2`, `deployment3`.

Running `kubectl backup deployment -n ns` would result in the creation of 3 yaml files in the current directory with the name of the deployments: `deployment1_deployment_ns.yaml`, `deployment2_deployment_ns.yaml`, `deployment3_deployment_ns.yaml`

# Naming

The saved object files are named as follow: NAME_TYPE_NAMESPACE.yaml. For example, `deployment1_deployment_ns.yaml`

if the resource is not namespaced the namespace is omitted. 


# Planned features: 

- allowing the generation of archives (zip/tar)
- adding the all namespaces flag `-all` for saving a resource from all namespaces