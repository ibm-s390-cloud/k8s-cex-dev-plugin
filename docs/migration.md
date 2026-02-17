# Migrating from kube-system to cex-device-plugin Namespace

This section describes how to move the CEX device plugin from the
`kube-system` namespace to its own namespace `cex-device-plugin`.

## Migration details

The migration basically corresponds to a re-installation of the CEX device
plugin and the corresponding CEX resource configuration in the new
`cex-device-plugin` namespace.

**NOTE:** To prevent resource conflicts when two plug-ins try to manage the same
device, you must first remove the plug-in from the `kube-system` namespace. This
will lead to a short interruption in service such that pods with containers
requesting a CEX resource cannot be scheduled. Already running containers with
allocated CEX resources will not be affected and continue to run.

To assist in the migration, download the deployment kustomize files from
[the CEX device plug-in github repository](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin)
and change to the download directory.  With these files it is possible to
install the CEX devce plug-in with a custom configuration in the new namespace
`cex-device-plugin`.

The configuration map is the only part that has to be moved.
Everything else can simply be deleted in the `kube-system` namespace
and freshly installed in the `cex-device-plugin` namespace.  The
kustomize templates for installation can be used to directly install
the CEX device plug-in with a custom configmap.

## Migration sequence

Follow these steps for migration:

1. Download the deployment Kustomize files from the
   [CEX device plug-in github repository](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin)
   and change into the download directory.

2. Copy the `cex_resources.json` file content contained in the configuration map
   in the `kube-system` namespace into `deployments/configmap/cex_resources.json`
   by running the following command:
   ```
   oc get cm -n kube-system cex-resources-config -o jsonpath="{.data['cex_resources\.json']}" > deployments/configmap/cex_resources.json
   ```
3. Remove the old CEX device plug-in daemonset from the `kube-system` namespace
   by running the following command:
   ```
   oc delete daemonset cex-plugin-daemonset -n kube-system
   ```
   Now, no new pods with containers requesting CEX devices can be
   created anymore.

4. Create the installation template by running the following command:
   ```
   oc create -k deployments/rhocp-create
   ```
   After successful installation, new pods with containers requesting
   CEX devices can be created and the CEX device plug-in should be
   fully functional again.

5. Optionally, the `cex-resources-config` configmap in the
   `kube-system` namespace can be cleaned up.  It is no longer needed
   since the new copy in the `cex-device-plugin` namespace is used.
   To delete the old configmap, run the following command:
   ```
   oc delete cm cex-resources-config -n kube-system
   ```
