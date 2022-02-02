# Installing and Configuring the CEX device plug-in {: #installing-and-configuring-the-cex-device-plug-in}

## Obtaining the CEX device plug-in {: #obtaining-the-cex-device-plug-in}

The sources of the CEX device plug-in are located on github:

[https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin)

* To use the certified and supported image from the Red Hat registry, run:

  `podman pull registry.connect.redhat.com/ibm/ibm-cex-device-plugin-cm:<version>`

* To use the community version, run:

  `podman pull quay.io/ibm/ibm-cex-plugin-cm:<version>`

The CEX device plug-in comprises several Golang source files that are built into one static
binary, which is embedded into a container image. A sample *Dockerfile* is provided in the git repository to build the Go code and package the binary into
a container image.

Next the container image needs to pushed into the image repository of your
Kubernetes cluster. This step highly depends on the actual cluster and the cluster
configuration and thus is not covered in this documentation.

## Installing the CEX device plug-in {: #installing-the-cex-device-plug-in}

The CEX device plug-in image needs to be run on each compute node with administrator
privileges. Kubernetes uses the concept of a *DaemonSet* for this kind of
cluster-wide service. The git repository shows a sample daemonset yaml file, which provides all the needed settings and options to run the CEX device plug-in as
Kubernetes daemonset. A working sample is provided in the appendix [Sample CEX device plug-in daemonset yaml](appendix.md#sample-cex-device-plug-in-daemonset-yaml).

To successfully run the CEX device plug-in the daemonset yaml, consider:

- `namespace`: The CEX device plug-in instances need to run in namespace
  `kube-system` as this is the same namespace where the CEX ConfigMap
  resides.
- `securityContext`: Must be *privileged* because the plug-in code needs access to some directories and files on the compute node:
  -  To establish an IPC connection to the kubelet.
  -  To do administrative tasks. For example, create and destroy zcrypt additional device nodes.
  -  To build and provide directory trees to be mounted into the client containers. For example, shadow sysfs.
- `volumes`: The plug-in needs some volumes from the compute node:
  - `/dev` and `/sys` are needed to access the zcrypt device node and
    to add and remove *zcrypt additional device nodes* to be used by the
    crypto load containers.
  - The device-plug-in API provided by Kubernetes is accessed via gRPC, which
    needs the directory `/var/lib/kubelet/device-plugins`.
  - The CEX ConfigMap is accessed as a volume, which provides one file
    `cex_resources.json` where the cluster-wide CEX configuration is
    stored.
  - Access to `/var/tmp` is needed to build up the sysfs overlay
    directories for each container that uses crypto resources. For details on
    sysfs overlay see: [The shadow sysfs](technical_concepts_limitations.md#the-shadow-sysfs).
- `InitContainer`: The commands needed to deal with SELinux have been
  outsourced into an InitContainer. The `InitContainer` entry can be safely removed
  from the yaml file when there is no need.

After obtaining the CEX device plug-in daemonset yaml file you should screen and maybe
update the plug-in image source registry and then apply it with the following command:

    kubectl create -f <my_cex_plug-in_daemonset.yaml>

A few seconds later a pod 'cex-plug-in' in namespace `kube-system` should run on every compute node.

## Further details on the CEX device plug-in {: #further-details-on-the-cex-device-plug-in}

A CEX device plug-in instance is an ordinary application built from Golang code. The
application provides a lot of information about what is
going on via stdout/stderr. You can generate the output with the `kubectl logs <pod>` command, which should contain the namespace `-n kube-system` option.

The CEX device plug-in application initially screens all the available APQNS on the
compute node, then reads in the CEX configuration. After verifying the CEX
configuration, a Kubernetes *device-plug-in* with the name of the config set is registered for each config set. This results in one device plug-in registration per config set with the full name *cex.s390.ibm.com/\<config-set-name\>*.
 * For details about Kubernetes device plug-in's see:
https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/

* For details about the Device Plugin Manager (dpm) see: https://pkg.go.dev/github.com/kubevirt/device-plugin-manager/pkg/dpm

After registration the CEX device plug-in is ready for allocation requests forwarded
from the kubelet service. Such an allocation request is triggered by a crypto
load pod requesting a CEX resource from the config set. The allocation request
is processed and creates:
- A new zcrypt device node and forwards it to the container.
- Sysfs shadow directories and makes sure they are mounted on to the
  correct place within the container.

In addition, there are some secondary tasks to do:
- APQN rescan: Every `APQN_CHECK_INTERVAL` (default is 30s) the
  available APQNs on the compute node are checked. When there are changes, the
  plug-in reevaluates the list of available APQNs per config set and reannounces
  the list of plug-in-devices to the Kubernetes system.
- CEX config map rescan: Every `CRYPTOCONFIG_CHECK_INTERVAL` (default is 120s) the crypto config map is re-read. If the verification of the ConfigMap
  succeeds, the changes are re-evaluated and eventually result in
  reannouncements to the Kubernetes system. If verification fails, an error
  message `Config Watcher: failed to verify new configuration!` is shown. The
  plug-in continues to run without CEX crypto configuration and is thus
  unable to satisfy allocation requests. For details see:
  [CEX configuration ConfigMap updates](technical_concepts_limitations.md#cex-configuration-configmap-updates).
- Surveillance of pods with CEX resources allocated: Every
  `PODLISTER_POLL_INTERVAL` (default is 30s) the list of pods, which have a
  CEX resource assigned, is examined. This is matched against the list of
  resources, which are provided by the plug-in. For each allocation request the
  plug-in creates a zcrypt device node and shadow sysfs directories. These
  resources must be removed when no longer needed:
  + When the resources (zcrypt device node, shadow sysfs directories), which
    were created based on an allocation request are not used any more (the pod
    using the related plug-in device has not been seen any more) for more than
    `RESOURCE_DELETE_UNUSED` (default is 120s) seconds, these resources are
    destroyed.
  + When a zcrypt device node and the shadow sysfs directories, which were
    created based on an allocation request have not been used (there was never
    seen a running pod with the related plug-in device) for more than
    `RESOURCE_DELETE_NEVER_USED` (default 1800s) seconds, the zcrypt
    device node and the shadow sysfs directories are destroyed.
