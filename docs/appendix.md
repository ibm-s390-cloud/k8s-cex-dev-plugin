# Appendix {: #appendix}

## Sample CEX resource configuration map {: #sample-cex-resource-configuration-map}

    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: cex-resources-config
      namespace: kube-system
    data:
      cex_resources.json: |
        {
        "cryptoconfigsets":
        [
            {
                "setname":   "CCA_for_customer_1",
                "project":   "customer-1",
                "cexmode":   "cca",
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

## Sample CEX device plug-in daemonset yaml {: #sample-cex-device-plug-in-daemonset-yaml}


    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
      name: cex-plug-in-daemonset
      namespace: kube-system
    spec:
      selector:
        matchLabels:
          name: cex-plug-in
      template:
        metadata:
          annotations:
            scheduler.alpha.kubernetes.io/critical-pod: ""
          labels:
            name: cex-plug-in
        spec:
          tolerations:
          - key: CriticalAddonsOnly
            operator: Exists
          initContainers:
          - name: shadowsysfs
            image: 'registry.access.redhat.com/ubi8/ubi-minimal'
            command: ["/bin/sh"]
            args: ["-c", "mkdir -p -m 0755 /var/tmp/shadowsysfs && chcon -t container_file_t /var/tmp/shadowsysfs"]
            securityContext:
              privileged: true
            volumeMounts:
              - name: vartmp
                mountPath: /var/tmp
          containers:
          - image: 'image-registry.openshift-image-registry.svc:5000/kube-system/cex-plugin-v1:latest'
            imagePullPolicy: Always
            name: cex-plug-in
            securityContext:
              privileged: true
            env:
              # provide NODENAME to the container
              - name: NODENAME
                valueFrom:
                  fieldRef:
                    fieldPath: spec.nodeName
            volumeMounts:
              - name: device-plug-in
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
            # device-plug-in gRPC needs this
            - name: device-plug-in
              hostPath:
                path: /var/lib/kubelet/device-plugins
            # pod-resources lister gRPC needs this
            - name: pod-resources
              hostPath:
                path: /var/lib/kubelet/pod-resources
            # plug-in shadow sysfs mounts need this
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

## Sample CEX crypto load container {: #sample-cex-crypto-load-container}


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

## Sample CEX quota restriction script {: #sample-cex-quota-restriction-script}


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
            for s in `oc get cm cex-resources-config -n kube-system -o jsonpath='{.data.cex_resources\.json}'
                      | jq -r ".cryptoconfigsets | .[] | select(.project != \"$n\") | .setname"`; do
            c=$(( c + 1 ))
            createquota $n $s $c
        done
        echo "kind: List" >> quota-$n.yaml
        echo "metadata: {}" >> quota-$n.yaml
        ## TODO: apply it
    done
