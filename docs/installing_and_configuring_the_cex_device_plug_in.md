# Installing and Configuring the CEX device plug-in

## Obtaining the CEX device plug-in

The sources of the CEX device plug-in are located on github:

[https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin)

* To use the certified and supported image from the Red Hat registry, run:

  `podman pull registry.connect.redhat.com/ibm/ibm-cex-device-plugin-cm:<version>`

* To use the community version, run:

  `podman pull quay.io/ibm/ibm-cex-plugin-cm:<version>`

The CEX device plug-in comprises several Golang source files that are built into
one static binary, which is embedded into a container image. A sample
*Dockerfile* is provided in the git repository to build the Go code and package
the binary into a container image.

Next the container image needs to pushed into the image repository of your
Kubernetes cluster. This step highly depends on the actual cluster and the cluster
configuration and thus is not covered in this documentation.

## Installing the CEX device plug-in

For installation of the CEX device plug-in, a set of Kustomize overlays is
provided in the `deployments` directory of the repository.
The source tree contains overlays for installation on Red Hat OpenShift
Container Platform in the following directories:
- `rhocp-create` a configmap
- `rhocp-update` an overlay to update the installation without touching an
  existing configmap
- `configmap` an overlay to only update or create a configmap
OpenShift Container Platform including a configmap (directory
`rhocp-create`), an overlay to update the installation without
touching a (possibly existing) configmap (directory `rhocp-update`), and
an overlay to only update or create a configmap (directory `configmap`).
See [the getting started
documentation](getting_started_with_the_cex_device_plug_in.md) for
details on the configuration.

To install the CEX device plug-in via these overlays with an empty
configmap, run the following command:
```
oc create -k deployments/rhocp-create
```

By default, this deploys an empty configuration map. For example, it results
in a CEX device plug-in that will not provide any devices. To
directly create a custom configuration map, edit the file
`cex_resources.json` in the `deployments/configmap` directory before
running the above command. See [Getting started with the CEX device plug-in]
(getting_started_with_the_cex_device_plug_in.md) for details.
documentation](getting_started_with_the_cex_device_plug_in.md) for
details on the configuration and how to update the configuration.

## Updating an Installation

If a configuration already exists in the `cex-device-plugin`
namespace, it should not be overwritten by the installation script.
To only update the CEX device plug-in, run the following command:
```
oc apply -k deployments/rhocp-update
```
This will update the cluster to the latest
version of the CEX device plug-in without changing the existing configuration.

## Installing the CEX device plug-in in details

The CEX device plug-in container image needs to be run privileged on
each compute node. Kubernetes uses the concept of a *DaemonSet* for
this kind of cluster-wide service. The git repository provides
kustomize-based deployments for installation on Red Hat OpenShift
Container Platform (directory `deployments/rhcop-create`).

To successfully run the CEX device plug-in the daemonset yaml, consider:

- `namespace`: The CEX device plug-in instances need to run in the
  same namespace where the CEX ConfigMap resides.
- `securityContext`: Must be *privileged* because the plug-in code needs access
  to some directories and files on the compute node:
  -  To establish an IPC connection to the kubelet.
  -  To do administrative tasks. For example, create and destroy zcrypt
     additional device nodes.
  -  To build and provide directory trees to be mounted into the client
     containers. For example, shadow sysfs.
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
- `InitContainer`: These commands set the appropriate SELinux labels for the
  shadow sysfs directory.  Required only for nodes that are enabled for SELinux.
- `serviceAccount` and `serviceAccountName` should point to the account running
  the containers in the pods. This account requires on a few privileges:
  - It requires `get`, `list`, and `watch` access to `pods` to keep track of
    pods using devices provided by the plugin.
  - It requires `get`, `list`, and `watch` access to `configmaps` to be able to
    update its own configuration.
  - It also requires `use` access to the `privileged` SCC.  This part is
    specific to RHOCP.

After obtaining the CEX device plug-in deployment files you should screen and
maybe update the plug-in image source registry and then apply it with the
following command:
```
oc create -k <kustomize_directory>
```

Here, `<kustomize_directory>` is the path to the desired directory you want to use.

A few seconds later a pod whose name starts with 'cex-plugin' in
namespace `cex-device-plugin` should run on every compute node.

## Further details on the CEX device plug-in

A CEX device plug-in instance is an ordinary application built from Golang
code. The application provides a lot of information about what is going on via
stdout/stderr. You can generate the output with the `kubectl logs <pod>`
command, which should contain the namespace `-n cex-device-plugin` option.

The CEX device plug-in application initially screens all the available APQNS on
the compute node, then reads in the CEX configuration. After verifying the CEX
configuration, a Kubernetes *device-plug-in* with the name of the config set is
registered for each config set. This results in one device plug-in registration
per config set with the full name *cex.s390.ibm.com/\<config-set-name\>*.
 * For details about Kubernetes device plug-in's see:
https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/

* For details about the Device Plugin Manager (dpm) see: https://pkg.go.dev/github.com/kubevirt/device-plugin-manager/pkg/dpm

After registration the CEX device plug-in is ready for allocation requests
forwarded from the kubelet service. Such an allocation request is triggered by a
crypto load pod requesting a CEX resource from the config set. The allocation
request is processed and creates:
- A new zcrypt device node and forwards it to the container.
- Sysfs shadow directories and makes sure they are mounted on to the
  correct place within the container.

In addition, there are some secondary tasks to do:
- APQN rescan: Every `APQN_CHECK_INTERVAL` (default is 30s) the available APQNs
  on the compute node are checked. When there are changes, the plug-in
  reevaluates the list of available APQNs per config set and reannounces the
  list of plug-in-devices to the Kubernetes system.
- CEX config map rescan: Every `CRYPTOCONFIG_CHECK_INTERVAL` (default is 120s)
  the crypto config map is re-read. If the verification of the ConfigMap
  succeeds, the changes are re-evaluated and eventually result in
  reannouncements to the Kubernetes system. If verification fails, an error
  message `Config Watcher: failed to verify new configuration!` is shown. The
  plug-in continues to run without CEX crypto configuration and is thus unable
  to satisfy allocation requests. For details see:
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
