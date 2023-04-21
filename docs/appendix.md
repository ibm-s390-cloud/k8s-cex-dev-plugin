# Appendix

## Sample CEX resource configuration map

    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: cex-resources-config
      namespace: cex-device-plugin
    data:
      cex_resources.json: |
        {
        "cryptoconfigsets":
        [
            {
                "setname":   "CCA_for_customer_1",
                "project":   "customer-1",
                "cexmode":   "cca",
                "overcommit": 3,
                "apqns":
                [
                    {
                        "adapter":    1,
                        "domain":     6,
                        "machineid": ""
                    },
                    {
                        "adapter":    2,
                        "domain":     6,
                        "machineid": ""
                    },
                    {
                        "adapter":    7,
                        "domain":     6,
                        "machineid": ""
                    }
                ]
            },
            {
                "setname":   "CCA_for_customer_2",
                "project":   "customer-2",
                "cexmode":   "cca",
                "overcommit": 4,
                "apqns":
                [
                    {
                        "adapter":    1,
                        "domain":    11,
                        "machineid": ""
                    },
                    ...
                    {
                        "adapter":    7,
                        "domain":    11,
                        "machineid": ""
                    }
                ]
            },
            {
                "setname":   "EP11_for_customer_1",
                "project":   "customer-1",
                "cexmode":   "ep11",
                "apqns":
                [
                    {
                        "adapter":    3,
                        "domain":     6,
                        "machineid": ""
                    },
                    ...
                    {
                        "adapter":   11,
                        "domain":     6,
                        "machineid": ""
                    }
                ]
            },
            {
                "setname":   "EP11_for_customer_2",
                "project":   "customer-2",
                "cexmode":   "ep11",
                "apqns":
                [
                    {
                        "adapter":    3,
                        "domain":    11,
                        "machineid": ""
                    },
                    ...
                    {
                        "adapter":   11,
                        "domain":    11,
                        "machineid": ""
                    }
                ]
            },
            {
                "setname":   "Accel",
                "project":   "default",
                "cexmode":   "accel",
                "overcommit": 5,
                "apqns":
                [
                    {
                        "adapter":    4,
                        "domain":     6,
                        "machineid": ""
                    },
                    ...
                    {
                        "adapter":    5,
                        "domain":     6,
                        "machineid": ""
                    }
                ]
            }
        ]
        }

## Sample CEX device plug-in daemonset yaml

    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
      name: cex-plugin-daemonset
      namespace: cex-device-plugin
    spec:
      selector:
        matchLabels:
          name: cex-plugin
      template:
        metadata:
          labels:
            name: cex-plugin
        spec:
          priorityClassName: system-cluster-critical
          serviceAccount: cex-plugin-sa
          serviceAccountName: cex-plugin-sa
          tolerations:
          - key: CriticalAddonsOnly
            operator: Exists
          # This Init Container defines settings for SELinux to enable the plug-in
          # to provide and modify a temporary file system. The file system can be
          # used to modify some sysfs entries for the container that uses CEX crypto
          # resources.  If the compute nodes do not have SELinux enabled, the Init
          # Container is not needed.
          initContainers:
          - name: shadowsysfs
            image: 'registry.redhat.io/ubi8-minimal'
            command: ["/bin/sh"]
            args: ["-c", "mkdir -p -m 0755 /var/tmp/shadowsysfs && chcon -t container_file_t /var/tmp/shadowsysfs"]
            securityContext:
              privileged: true
            volumeMounts:
              - name: vartmp
                mountPath: /var/tmp
          containers:
          - name: cex-plugin
            image: 'quay.io/ibm/ibm-cex-plugin-cm:latest'
            imagePullPolicy: Always
            securityContext:
              privileged: true
            command: ["/work/cex-plugin"]
            env:
              # provide NODENAME to the container
              - name: NODENAME
                valueFrom:
                  fieldRef:
                    fieldPath: spec.nodeName
              # logically overcommit (share) CEX resources (if >1)
              - name: APQN_OVERCOMMIT_LIMIT
                value: "1"
            volumeMounts:
              - name: device-plugin
                mountPath: /var/lib/kubelet/device-plugins
              - name: pod-resources
                mountPath: /var/lib/kubelet/pod-resources
              - name: vartmp
                mountPath: /var/tmp
              - name: dev
                mountPath: /dev
              - name: sys
                mountPath: /sys
              - name: cex-resources-conf
                # the cex_resources.json file is showing up in this dir
                mountPath: /config/
          volumes:
            # device-plugin gRPC needs this
            - name: device-plugin
              hostPath:
                path: /var/lib/kubelet/device-plugins
            # pod-resources lister gRPC needs this
            - name: pod-resources
              hostPath:
                path: /var/lib/kubelet/pod-resources
            # plugin shadow sysfs mounts need this
            - name: vartmp
              hostPath:
                path: /var/tmp
            - name: dev
              hostPath:
                path: /dev
            - name: sys
              hostPath:
                path: /sys
            # cluster wide crypto cex resources config
            - name: cex-resources-conf
              configMap:
                name: cex-resources-config

## Sample CEX crypto load container

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: testload-cca-for-customer-1
      namespace: customer-1
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: testload-cca-for-customer-1
      strategy:
        type: Recreate
      template:
        metadata:
          labels:
            app: testload-cca-for-customer-1
        spec:
          containers:
          - image: 'bash'
            imagePullPolicy: Always
            name: testload-cca-for-customer-1
            command: ["/bin/sh", "-c", "while true; do echo do-nothing-loop; sleep 30; done"]
            resources:
              limits:
                cex.s390.ibm.com/CCA_for_customer_1: 1

## Sample CEX quota restriction script

    #!/bin/bash

    # This script produces a yaml file with quota restrictions
    # for the cex cryptosets for each given namespace.
    # Apply the resulting yaml file and then only the namespace <nnn>
    # is allowed  to allocate CEX resources from a crypto set
    # marked with project <nnn>.

    createquota () {
        QF=quota-$1.yaml
        cat << EOF >> $QF
    - apiVersion: v1
      kind: ResourceQuota
      metadata:
        name: cex.$3
        namespace: $1
      spec:
        hard:
          requests.cex.s390.ibm.com/$2: 0
          limits.cex.s390.ibm.com/$2: 0
    EOF
    }

    while ! test -z "$1"; do
        n=$1
        shift
        c=0
        echo "apiVersion: v1" > quota-$n.yaml
        echo "items:" >> quota-$n.yaml
            for s in `oc get cm cex-resources-config -n cex-device-plugin -o jsonpath='{.data.cex_resources\.json}'
                      | jq -r ".cryptoconfigsets | .[] | select(.project != \"$n\") | .setname"`; do
            c=$(( c + 1 ))
            createquota $n $s $c
        done
        echo "kind: List" >> quota-$n.yaml
        echo "metadata: {}" >> quota-$n.yaml
        ## TODO: apply it
    done

## Sample CEX Prometheus exporter yaml

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: cex-prometheus-exporter
      namespace: cex-device-plugin
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: cex-prometheus-exporter
      strategy:
        type: Recreate
      template:
        metadata:
          labels:
            app: cex-prometheus-exporter
        spec:
          serviceAccount: cex-prometheus-exporter-sa
          serviceAccountName: cex-prometheus-exporter-sa
          containers:
            - name: cex-prometheus-exporter
              image: 'quay.io/ibm/ibm-cex-plugin-cm:latest'
              imagePullPolicy: Always
              securityContext:
                allowPrivilegeEscalation: false
                capabilities:
                  drop: ["ALL"]
                runAsNonRoot: true
                seccompProfile:
                  type: RuntimeDefault
              command: ["/work/cex-prometheus-exporter"]
              ports:
                - containerPort: 9939
                  name: prommetrics
                - containerPort: 12358
                  name: collector

## Sample CEX Prometheus exporter collector service yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: cex-prometheus-exporter-collector-service
      namespace: cex-device-plugin
      labels:
        app: cex-prometheus-exporter
    spec:
      type: ClusterIP
      selector:
        app: cex-prometheus-exporter
      ports:
      - name: collector
        port: 12358
        protocol: TCP
        targetPort: collector

## Sample CEX Prometheus exporter servicemonitor yaml

    apiVersion: monitoring.coreos.com/v1
    kind: ServiceMonitor
    metadata:
      name: cex-prometheus-exporter
      namespace: cex-device-plugin
      labels:
        release: prometheus
    spec:
      selector:
        matchLabels:
          app: cex-prometheus-exporter
      endpoints:
      - port: metrics
        interval: 15s
        scheme: http

## Sample CEX Prometheus exporter service yaml

    apiVersion: v1
    kind: Service
    metadata:
      name: cex-prometheus-exporter
      namespace: cex-device-plugin
      labels:
        app: cex-prometheus-exporter
    spec:
      type: ClusterIP
      selector:
        app: cex-prometheus-exporter
      ports:
      - name: metrics
        port: 9939
        protocol: TCP
        targetPort: prommetrics

## Environment variables

### Environment variables recognized by the CEX plug-in application

| Name | Default value | Description |
|:-----|:--------------|:-------|
`APQN_CHECK_INTERVAL` | `30` | The interval in seconds to check for the node APQNs available and their health state. The minimum is 10 seconds.
`APQN_OVERCOMMIT_LIMIT` | `1` | The overcommit limit, `1` defines no overcommit. For details see [Overcommitment of CEX resources](technical_concepts_limitations.md#overcommitment-of-cex-resources)
`CEX_PROM_EXPORTER_COLLECTOR_SERVICE_NAMESPACE` | | The namespace in which the CEX Prometheus exporter will run. If empty (the default) it is assumed that CEX plug-in instances and the CEX Prometheus exporter run in the same namespace.
`CEX_PROM_EXPORTER_COLLECTOR_SERVICE_PORT` | `12358` | The port number where the CEX plug-in instances will contact the CEX Prometheus exporter to deliver their raw metrics data.
`CEX_PROM_EXPORTER_COLLECTOR_SERVICE` | `cex-prometheus-exporter-collector-service` | The name of the service where the CEX plug-in instance will contact the CEX Prometheus exporter.
`CRYPTOCONFIG_CHECK_INTERVAL` | `120` | The interval in seconds to check for changes on the cluster-wide CEX resource configmap. The minimum is 120 seconds.
`METRICS_POLL_INTERVAL` | `15` | The interval in seconds to internally poll base information (like crypto counters) and update the internal metrics data. The minimum is 10 seconds.
`NODENAME` | | The name of the node where the CEX device plug-in instance runs. See the sample CEX plug-in daemonset yaml to set up this environment variable correctly.
`PODLISTER_POLL_INTERVAL` | `30` | The interval in seconds to fetch and evaluate the pods within the cluster, which have CEX resources allocated. The minimum is 10 seconds.
`RESOURCE_DELETE_NEVER_USED` | `1800` | The interval in seconds after which an allocated CEX resource requested by a starting pod is freed when the pod never came into the running state. The minimum is 30 seconds.
`RESOURCE_DELETE_UNUSED` | `120` | The interval in seconds after which an allocated CEX resource is freed when the pod vanished from the running pods list. The minimum is 30 seconds.
`SHADOWSYSFS_BASEDIR` | `/var/tmp/shadowsysfs` | The base directory for the shadow sysfs. For details see [The shadow sysfs](technical_concepts_limitations.md#the-shadow-sysfs)

### Environment variables recognized by the CEX Pometheus exporter application

| Name | Default value | Description |
|:-----|--------------:|:-------|
`COLLECTOR_SERVICE_PORT`  | `12358` | The metrics collector listener port, where the CEX plug-in instances will deliver their raw metrics data.
`PROMETHEUS_SERVICE_PORT` |  `9939` | The Prometheus client port where the Prometheus server will fetch the metrics from.
