# Troubleshooting {: #troubleshooting}

This section provides information on diagnostics and troubleshooting.

If you experience issues with the CEX device plug-in, you can check
the pod status, gather pod diagnostics, and collect debugging data.

### Prerequisites

You must log in as a user that belongs to a role with administrative privileges for the cluster.
For example, `system:admin` or `kube:admin`.

The CEX device plug-in runs as a daemonset in namespace
`kube-system`. This query should list the CEX device plug-in
daemonset:

    $ kubectl get daemonsets -n kube-system
    NAME                   DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
    cex-plugin-daemonset   3         3         3       3            3           <none>          4d

The daemonset is realized as a pod with one container per each compute
node. The following command should list the pods of the CEX device
plug-in, and maybe other pods running in the kube-system namespace as
well:

    $ kubectl get pods -n kube-system
    NAME                         READY   STATUS    RESTARTS   AGE
    cex-plugin-daemonset-bfxt2   1/1     Running   0          3d23h
    cex-plugin-daemonset-bhhj8   1/1     Running   0          3d23h
    cex-plugin-daemonset-bntsp   1/1     Running   0          3d23h

Verify that the pods are running correctly. There should be one pod
per each compute node in state `Running`. If one or more of the CEX
device plug-in pods do not show up or are not showing a Running
status, you can collect diagnostic information. To inspect the status
of a pod in detail, use the `describe` subcommand, for example:

    $ kubectl describe pod cex-plugin-daemonset-bfxt2 -n kube-system

When these requirements are fulfilled, ensure that you have a CEX
resource configuration map, which defines the *CEX config sets*
deployed in namespace `kube-system`.

    $ kubectl get configmap -n kube-system
    NAME                                 DATA   AGE
    ...                                  ...    ...
    cex-resources-config                 1      4d2h
    ...                                  ...    ...

When there is no configmap deployed a `kubectl describe` on one of the
CEX devie plug-in pods will show a message that the volume mount
failed, about like this:

    MountVolume.SetUp failed for volume "cex-resources-conf" : configmap "cex-resources-config" not found

If the CEX configmap is deployed and the CEX device plug-in instances
are running, verify the available and allocated CEX resources on each
compute node:

    $ kubectl describe nodes
    ...
    Allocatable:
      ...
      cex.s390.ibm.com/Accel:                1
      cex.s390.ibm.com/CCA_for_customer_1:   3
      cex.s390.ibm.com/EP11_for_customer_2:  2
      cpu:                                   3500m
      ephemeral-storage:                     15562841677
      ...
    ...
    Allocated resources:
      Resource                              Requests      Limits
      --------                              --------      ------
      cpu                                   408m (11%)    0 (0%)
      memory                                2213Mi (20%)  0 (0%)
      ephemeral-storage                     0 (0%)        0 (0%)
      hugepages-1Mi                         0 (0%)        0 (0%)
      cex.s390.ibm.com/Accel                0             0
      cex.s390.ibm.com/CCA_for_customer_1   1             1
      cex.s390.ibm.com/EP11_for_customer_2  0             0
      ...

Each CEX device plug-in pod provides log messages, which provide
details that might explain a possible failure or misbehavior. The logs
of each of the CEX device plug-in instances may be extracted with a
command sequence like this:

    $ kubectl get pods -n kube-system --no-headers | grep cex-plugin-daemonset
    cex-plugin-daemonset-p5j8h   1/1   Running   0     32m
    cex-plugin-daemonset-qdz8r   1/1   Running   0     32m
    cex-plugin-daemonset-zxwts   1/1   Running   0     32m
    $ kubectl logs -n kube-system cex-plugin-daemonset-p5j8h
    $ kubectl logs -n kube-system cex-plugin-daemonset-qdz8r
    $ kubectl logs -n kube-system cex-plugin-daemonset-zxwts

Here are some important parts of a sample CEX device plug-in log shown with some explanations:

     1: 2022/06/07 14:05:18 Main: S390 k8s z crypto resources plugin starting
     2: 2022/06/07 14:05:18 Plugin Version: v1.0.2
     3: 2022/06/07 14:05:18 Git URL:        https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin.git
     4: 2022/06/07 14:05:18 Git Commit:     40fae46c3d3aacff055d5f2fd7e1c580abc850b9

Line 2: CEX device plug-in version.

Line 3-4: Source code and commit id base for this CEX device plug-in application.

     5: 2022/06/07 14:05:18 Main: Machine id is 'IBM-3906-00000000000DA1E7'
     6: 2022/06/07 14:05:18 Ap: apScanAPQNs() found 4 APQNs: (6,51,cex6,accel,true), (8,51,cex6,cca,true), (9,51,cex6,cca,true), (10,51,cex6,ep11,true)
     7: 2022/06/07 14:05:18 CryptoConfig: Configuration changes detected
     8: 2022/06/07 14:05:18 CryptoConfig: Configuration successful updated
     9: 2022/06/07 14:05:18 Main: Crypto configuration successful read
    10: 2022/06/07 14:05:18 CryptoConfig (3 CryptoConfigSets):
    11: 2022/06/07 14:05:18   setname: 'CCA_for_customer_1'
    12: 2022/06/07 14:05:18     project: 'customer_1'
    13: 2022/06/07 14:05:18     5 equvialent APQNs:
    14: 2022/06/07 14:05:18       APQN adapter=4 domain=51 machineid='*'
    15: 2022/06/07 14:05:18       APQN adapter=8 domain=51 machineid='*'
    16: 2022/06/07 14:05:18       APQN adapter=9 domain=51 machineid='*'
    17: 2022/06/07 14:05:18       APQN adapter=12 domain=51 machineid='*'
    18: 2022/06/07 14:05:18       APQN adapter=13 domain=51 machineid='*'
    19: 2022/06/07 14:05:18   setname: 'EP11_for_customer_2'
    20: 2022/06/07 14:05:18     project: 'customer_1'
    21: 2022/06/07 14:05:18     3 equvialent APQNs:
    22: 2022/06/07 14:05:18       APQN adapter=5 domain=51 machineid='*'
    23: 2022/06/07 14:05:18       APQN adapter=10 domain=51 machineid='*'
    24: 2022/06/07 14:05:18       APQN adapter=11 domain=51 machineid='*'
    25: 2022/06/07 14:05:18   setname: 'Accel'
    26: 2022/06/07 14:05:18     project: 'default'
    27: 2022/06/07 14:05:18     3 equvialent APQNs:
    28: 2022/06/07 14:05:18       APQN adapter=3 domain=51 machineid='*'
    29: 2022/06/07 14:05:18       APQN adapter=6 domain=51 machineid='*'
    30: 2022/06/07 14:05:18       APQN adapter=7 domain=51 machineid='*'

Line 6: The list of APQNs found by the CEX device plug-in instance on the compute node.

Lines 10-30: Condensed view of the CEX resource configuration.

    ...
    40: 2022/06/07 14:05:18 PodLister: Start()
    41: 2022/06/07 14:05:18 Plugin: Register plugins for these CryptoConfigSets: [Accel CCA_for_customer_1 EP11_for_customer_2]
    42: 2022/06/07 14:05:18 Plugin: Announcing 'cex.s390.ibm.com' as our resource namespace
    43: 2022/06/07 14:05:18 Plugin: NewPlugin('EP11_for_customer_2')
    44: 2022/06/07 14:05:18 Plugin['EP11_for_customer_2']: Start()
    45: 2022/06/07 14:05:18 Plugin: Announcing 'cex.s390.ibm.com' as our resource namespace
    46: 2022/06/07 14:05:18 Plugin: NewPlugin('Accel')
    47: 2022/06/07 14:05:18 Plugin['Accel']: Start()
    48: 2022/06/07 14:05:18 Plugin: Announcing 'cex.s390.ibm.com' as our resource namespace
    49: 2022/06/07 14:05:18 Plugin: NewPlugin('CCA_for_customer_1')
    50: 2022/06/07 14:05:18 Plugin['CCA_for_customer_1']: Start()
    51: 2022/06/07 14:05:18 Plugin['Accel']: Found 1 eligible APQNs: (6,51,cex6,accel,true)
    52: 2022/06/07 14:05:18 Plugin['Accel']: Overcommit not specified in ConfigSet, fallback to 1
    53: 2022/06/07 14:05:18 Plugin['Accel']: Derived 1 plugin devices from the list of APQNs
    54: 2022/06/07 14:05:18 Plugin['EP11_for_customer_2']: Found 1 eligible APQNs: (10,51,cex6,ep11,true)
    55: 2022/06/07 14:05:18 Plugin['EP11_for_customer_2']: Overcommit not specified in ConfigSet, fallback to 1
    56: 2022/06/07 14:05:18 Plugin['EP11_for_customer_2']: Derived 1 plugin devices from the list of APQNs
    57: 2022/06/07 14:05:18 Plugin['CCA_for_customer_1']: Found 2 eligible APQNs: (8,51,cex6,cca,true), (9,51,cex6,cca,true)
    58: 2022/06/07 14:05:18 Plugin['CCA_for_customer_1']: Overcommit not specified in ConfigSet, fallback to 1
    59: 2022/06/07 14:05:18 Plugin['CCA_for_customer_1']: Derived 2 plugin devices from the list of APQNs
    ...

Lines 51, 54, 57: List of APQNs from the different CEX config sets
that have been found on the compute node and are allocatable.

The following example shows a real allocation by a container:

    ...
    70: 2022/06/07 14:17:03 Plugin['CCA_for_customer_1']: Allocate(request=&AllocateRequest{ContainerRequests:
			    []*ContainerAllocateRequest{&ContainerAllocateRequest{DevicesIDs:[apqn-9-51-0],},},})
    71: 2022/06/07 14:17:03 Plugin['CCA_for_customer_1']: creating zcrypt device node 'zcrypt-apqn-9-51-0'
    72: 2022/06/07 14:17:03 Zcrypt: Successfully created new zcrypt device node 'zcrypt-apqn-9-51-0'
    73: 2022/06/07 14:17:03 Zcrypt: simple node 'zcrypt-apqn-9-51-0' for APQN(9,51) created
    74: 2022/06/07 14:17:03 Shadowsysfs: shadow dir /var/tmp/shadowsysfs/sysfs-apqn-9-51-0 created
    75: 2022/06/07 14:17:03 Plugin['CCA_for_customer_1']: Allocate() response=&AllocateResponse{ContainerResponses:
			    []*ContainerAllocateResponse{&ContainerAllocateResponse{Envs:map[string]string{},Mounts:[]*Mount{&Mount{ContainerPath:/sys/bus/ap,HostPath:/var/tmp/shadowsysfs/sysfs-apqn-9-51-0/bus/ap,ReadOnly:true,},&Mount{ContainerPath:/sys/devices/ap,HostPath:/var/tmp/shadowsysfs/sysfs-apqn-9-51-0/devices/ap,ReadOnly:true,},},Devices:[]*DeviceSpec{&DeviceSpec{ContainerPath:/dev/z90crypt,HostPath:/dev/zcrypt-apqn-9-51-0,Permissions:rw,},},Annotations:map[string]string{},},},}
    ...

About every 30 seconds the list of running containers with allocated
CEX resources is listed:

    ...
    80: 2022/06/07 14:47:18 PodLister: 1 active zcrypt nodes
    81: 2022/06/07 14:47:18 PodLister: 1 active sysfs shadow dirs
    82: 2022/06/07 14:47:18 PodLister: Container 'cex-testload-1' in namespace 'default' uses CEX resource 'apqn-9-51-0' marked for project 'customer_1'!!!
    83: 2022/06/07 14:47:18 PodLister: 1 active containers with allocated cex devices
    ...

When containers terminate with an allocated CEX resource there is a
cleanup step which is reported in the log like this:

    ...
    90: 2022/06/07 14:52:18 PodLister: 1 active zcrypt nodes
    91: 2022/06/07 14:52:18 PodLister: 1 active sysfs shadow dirs
    92: 2022/06/07 14:52:18 PodLister: 0 active containers with allocated cex devices
    93: 2022/06/07 14:52:18 PodLister: deleting zcrypt node 'zcrypt-apqn-9-51-0': no container use since 120 s
    94: 2022/06/07 14:52:18 PodLister: deleting shadow sysfs 'sysfs-apqn-9-51-0': no container use since 120 s
    ...


## Capturing debug data for support {: capturing-debug-data-for-support}

If you submit a support case, provide debugging data. Describe the
failure and the expected behavior and collect the logs of **all** CEX
device plug-in instances together with the **currently active** CEX
resource configuration map. Optionally you can include the output of
`kubectl describe nodes`. Be careful when providing this node data as
internals of the load on the cluster might be exposed.

For example, run following commands to collect the required
information:

    $ cd /tmp
    $ for p in `kubectl get pods -n kube-system --no-headers | grep cex-plugin-daemonset | awk '{print $1}'`; do \
	kubectl logs -n kube-system $p >$p.log; done
    $ kubectl get configmap -n kube-system cex-resources-config -o yaml >cex-resources-config.yaml
    $ kubectl describe nodes >describe_nodes.log
    $ zip debugdata.zip cex-plugin-daemonset-*.log cex-resources-config.yaml describe_nodes.log
    $ rm cex-plugin-daemonset-*.log cex-resources-config.yaml describe_nodes.log

Note: The CEX device plug-in does not have access to any cluster or
application secrets. Therefore, only administrative information,
related to the APQNs that are managed by the plug-in is logged. The
logs contain the name of the configuration sets and the name and
namespace of pods that request and use APQNs. Since no application,
cluster, or company secrets are contained within the logs, it is safe
to hand over this logging information to the technical support.
