# Kubernetes device plugin for IBM CryptoExpress (CEX) cards

This publication provides information on version 1 of the Kubernetes device plugin for IBM
CryptoExpress (CEX) cards available for IBM Z and LinuxONE (s390x).

Authors:

- Harald Freudenberger <freude@linux.ibm.com>
- Juergen Christ <jchrist@linux.ibm.com>

Changelog:
- 2021/07/08: first draft version
- 2021/09/29: rework to reflect version 1

Table of Contents:
- [Kubernetes device plugin for IBM CryptoExpress (CEX) cards](#kubernetes-device-plugin-for-ibm-cryptoexpress-cex-cards)
  - [Inclusive language](#inclusive-language)
  - [Introduction](#introduction)
    - [Overview](#overview)
  - [Setting up the environment for the CEX device plugin](#setting-up-the-environment-for-the-cex-device-plugin)
    - [CEX resources on IBM Z and LinuxONE](#cex-resources-on-system-z)
  - [Installing the CEX device plugin](#installing-the-cex-device-plugin)
    - [Creating and Establishing a CEX Resource Configuration Map](#creating-and-establishing-a-cex-resource-configuration-map)
      - [Considerations for equally configured APQNs](#considerations-for-equally-configured-apqns)
      - [Basic parameters](#basic-parameters)
      - [APQN parameters](#apqn-parameters)
    - [Installing and Configuring the CEX device plugin](#installing-and-configuring-the-cex-device-plugin)
      - [Getting the CEX device plugin](#getting-the-cex-device-plugin)
      - [Installing the CEX device plugin](#installing-the-cex-device-plugin)
      - [Some more details on the CEX device plugin](#some-more-details-on-the-cex-device-plugin)
  - [Allocation of CEX resources by client containers](#allocation-of-cex-resources-by-client-containers)
  - [Details](#details)
    - [CEX configuration ConfigMap updates](#cex-configuration-configmap-updates)
    - [Over-commitment of CEX resources](#over-commitment-of-cex-resources)
    - [The device node z90crypt](#the-device-node-z90crypt)
    - [The shadow sysfs](#the-shadow-sysfs)
    - [Hot-Plug and Hot-Unplug of APQNs](#hot-plug-and-hot-unplug-of-apqns)
  - [Limitations](#limitations)
    - [Namespaces and the project field](#namespaces-and-the-project-field)
  - [Appendix](#appendix)
    - [Sample CEX resource configuration map](#sample-cex-resource-configuration-map)
    - [Sample CEX device plugin daemonset yaml](#sample-cex-device-plugin-daemonset-yaml)
    - [Sample CEX crypto load container](#sample-cex-crypto-load-container)
    - [Sample CEX quota restriction script](#sample-cex-quota-restriction-script)
  - [References](#references)
  - [Notices](#notices)
    - [Trademarks](#trademarks)
    - [Terms and conditions for product documentation](#terms-and-conditions-for-product-documentation)


## Inclusive language

While IBM values the use of inclusive language, terms that are outside of IBM's
direct influence are sometimes required for the sake of maintaining user
understanding. As other industry leaders join IBM in embracing the use of
inclusive language, IBM will continue to update the documentation to reflect
those changes.

To learn more about this initiative, read the [Words matter blog on
ibm.com](https://www.ibm.com/blogs/think/2020/08/words-matter-driving-thoughtful-change-toward-inclusive-language-in-technology/).


## Introduction

This Kubernetes device plugin provides access to IBM s390 Crypto Express (CEX)
cards for s390 Kubernetes container loads. Throughout this publication, the term
'CEX device plugin' is used to refer to this Kubernetes device plugin.


### Overview

The Kubernetes CEX device plugin provides IBM CryptoExpress cards to be made
available on Kubernetes node(s) for use by containers.

The CEX device plugin version 1 groups the available CEX resources (*APQNs*)
into *CEX config sets*. Containers can request **one** resource from **one**
*CEX config set*. Hence, from a container perspective, the APQNs within one CEX
config set should be equal. See (Considerations for equally configured
APQNs)[#considerations-for-equally-configured-APQNs] for some guidance on how to
set up a CEX config set. The CEX config sets are described in a cluster wide
*config map* which is maintained by the cluster administrator.

The CEX device plugin instances running on all compute nodes handle CEX
resource allocation requests from the containers, check if the existing crypto
resources are available on the nodes, claim the resource, and ensure containers
are scheduled on the right worker node with the requested CEX crypto resources.

So the main job of the CEX device plugin instances running on each worker node is to
- screen all the available CEX resources on the worker node and provide this
  information to the Kubernetes infrastructure service 
- and allocate and deallocate a CEX resource on request of the Kubernetes
  infrastructure based on the requirement of a pod asking for CEX support.

The application container on the other side only needs to specify that it needs a
CEX resource from a specific *CEX config set* with a Kubernetes resource limit
declaration. The cluster system together with the CEX device plugin handles the
details, claims a CEX resource and schedules the pod on the right worker node.

The following sections provide more information about CEX resources on IBM Z and LinuxONE,
the CEX device plugin details, the CEX crypto configuration as config sets in a cluster,
and application container handling details.


## Setting up the environment for the CEX device plugin

### CEX resources on IBM Z and LinuxONE

A IBM Z and LinuxONE machine can have multiple Crypto Express cards (short *CEX*) plugged
in. The Crypto Express Cards are available in several generations now. Supported
by the CEX device plugin is *CEX4* up to *CEX7*. For each card a mode of operation
needs to be chosen: *Accelerator* mode, *CCA mode* or *EP11* mode.

Each card is logically partitioned into independent units, so called crypto domains,
which represent independent HSMs (*Hardware Secure Module*s). All these
independent units within a card share the same mode of operation and the same
card generation.

So one could address one HSM unit with the *adapter* number (the crypto card
number within a machine) and the *domain* number (the crypto partition
index). These both numeric values in the range 0-255 can act as a link to one
HSM unit and are often called an *APQN*.

An LPAR within a IBM Z and LinuxONE machine can have one or more crypto cards assigned and
one or more domains. Tis results in a 2-dimensional table of APQNs.

The Kubernetes cluster is implemented as a KVM host running on a LPAR. The master
and worker nodes of the cluster are represented by KVM guests running at and
controlled by the KVM host. Therefore some or all of the crypto resources available
on the LPAR need to be provided for use of the KVM guests running as Kubernetes
worker nodes.

The point of view for an KVM guest running as a Kubernetes worker node is
similar to the view of the LPAR. A worker node might have zero or more crypto
adapters assigned and zero or more domains which can be seen as a 2-dimensional
table of APQNs.

This documentation does not cover the assignment and distribution of crypto
resources to LPARs, KVM hosts and KVM guests. For details about assigning crypto
resources to an LPAR see
[\[1\]](https://www.redbooks.ibm.com/abstracts/sg248860.html) chapter 10.1.3
"Configuring Crypto Express7S". More details about how to assign crypto
resources to KVM guests can be found in
[\[2\]](https://www.ibm.com/docs/en/linux-on-systems?topic=linux-z-linuxone)
chapter "Configuring Crypto Express Adapters for KVM Guests". More on Crypto
Express Cards, generations and operation modes can be found at
[\[3\]](https://www.ibm.com/security/cryptocards).

Usually the adapter/domain pair is sufficient to identify an APQN. But if the
worker nodes of a cluster are distributed over multiple IBM Z and LinuxONE machines
this is not the case any more. So there is the need to have a unique
*machine identification* (short *machine-id*) together with the adapter and
domain information.

An HSM contains a *secret* which must not get exposed to anybody. The secret, and
maybe additional settings of the HSM, is maintained by the *Security
Administrator* of the system. These settings are typically done out-of-band,
properly maintained and relatively static (changes every year or so). With a
IBM Z and LinuxONE all around crypto cards is typically done by the Security Administrator
with the help of a *TKE* (Trusted Key Entry). More about TKE can be found in
[\[6\]](https://www.redbooks.ibm.com/abstracts/sg247848.html).

The *secret* is usually the source of a secret key often referred to as the
*master key* or master wrapping key. Applications working with the HSM use
*secure key* objects which are in fact clear key values encrypted ("wrapped") by
the master key. Such a secure key can only be used together with the HSM as only
the HSM has the master key to unwrap the secure key blob.

A CEX crypto card in EP11 mode contains one *wrapping key*. A crypto card in CCA
mode contains 4 *master key*s, one to wrap DES keys, one for AES keys, one for
RSA keys and another one for ECC keys. A CEX card in accelerator mode does not
contain any secrets and can only be used to accelerate RSA clear key operations.

Multiple HSMs may be set up by the security administrator such that they can be
used as a backup of each other. So the master key(s) and maybe additional
settings may be *equal*. *Equal* here means that an application using secure key
methods may fulfill the job with either one of these HSMs - the HSMs form an
equivalence set. Spread these equal APQNs among the
worker nodes to allow the Kubernetes dispatching algorithm to choose the target
node of a crypto load based on several criteria like CPU and memory requirements
and the availability of crypto resources is just one requirement.

In version 1 of the CEX device plugin, it is not preferred to let a container change
the configuration (master key, control points) of its HSM resources. Also any
changes to crypto resources (mode, master keys, control points) should be
performed while the affected APQNs are not available for use within the cluster.

<!-- xxx z/VM is not described here in any way -->
<!-- xxx it would also be nice to have an example here with some drawings -->

## Installing the CEX device plugin

### Creating and Establishing a CEX Resource Configuration Map

The CEX device plugin needs to know a valid CEX configuration to
start up properly. So this chapter deals with creating an CEX
resource configuration.

In the CEX resource configuration, equivalent APQNs are grouped into equivalence
sets, here called *crypto config sets*. A *crypto config set* has a name and
comprises of one ore more equivalent APQNs. A pod with a crypto application
requests a CEX resource with requesting the allocation of one arbitrary APQN
from the config set by the name of the config set.

#### Considerations for equally configured APQNs

Within each configuration set, all the APQNs must be set up consistently. 
For each CEX mode, consider:
- For Common Cryptographic Architecture (CCA) CEX resources, the
  master keys and access control point settings should be equal.
- For EP11 CEX resources, EP11 wrapping key and control settings should be equal.
- CEX accelerator resources are stateless and do not need any equal setup.

A container requests exactly **one** configuration set and obtains
**one** CEX crypto resource by the CEX device plugin if one APQN is
available, healthy, and not already allocated. The APQN is randomly
choosen and is assigned to the life time of the container.

The clusterwide configuration of the CEX crypto resources crypto is
kept in a Kubernetes *ConfigMap* within the `kube-system`
namespace. The config map must be named `cex-resources-config`. The
content of the config map is a config file section in JSON format.

Here is a snippet showing only the head and some of the possibly many crypto
config set definitions. The fields will be discussed in more detail in the
following paragraph. Appendix
[Sample CEX resource Configuration Map](#sample-cex-resource-configuration-map)
shows a working sample for a real config map.

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
            ...

The ConfigMap defines a list of configuration sets. Each configuration
set comprises these entries:

#### Basic parameters

- `setname`: required, can be any string value, must be unique within
  all the configuration sets. This is the identifier used by the
  container to request one of the CEX crypto resources from within the
  set.
- `project`: required, can be any string value, namespace of the
  configuration set. Only containers with matching namespace can
  access CEX crypto resources of the configuration set. For version 1
  this is not fully implemented as there are limits on the existing
  API preventing this. More details about this can be found in
  [Limitations](#limitations).
- `cexmode`: optional, specifies the CEX mode. If specified, one of the
  following choices is required: `ep11`, `cca`, or `accel`.  
  Adds an extra verification step every time the APQNs on each node are screened
  by the CEX device plugin. All APQNs of the configuration set must match the
  specified CEX mode. On mismatches, the CEX device plugin creates a log entry
  and discards the use of this APQN for the configuration set.  
  TODO for v1.1
- `mincexgen`: optional, specifies the minimal CEX card generation for the
  configuation set. If specified, must match to `cex[4-7]`.  
  Adds an extra verification step every time the APQNs on each compute node are
  screened. All APQNs of the configuration set are checked to have at least the
  specified CEX card generation. On mismatches, the CEX device plugin creates a
  log entry and discards the use of the APQN for the configuration set.  
  TODO for v1.1

#### APQN parameters

- `apqns`: A list of equivalent APQN entries. What *equivalent* exactly means
  depends on the crypto workload to be run with this crypto config
  set. However, it forms a set of APQNs where anyone is sufficient to fulfill
  the needs of the requesting crypto workload container (see also
  [Considerations for equally configured APQNs](#considerations-for-equally-configured-apqns)
  For example a CCA application using the very same AES secure key each time
  relies on APQNs with a matching AES master key for this AES secure key. In
  other words the crypto setup of the APQNs should be the same (e.g. here in
  terms of the AES master key).
  
<!-- RB: to be discussed
 delete first sentence An APQN ... config set.
-->
  An APQN must not be member of more than one crypto config set. It is valid to
  provide an empty list here. It is also valid to provide APQNs here which may
  currently not exist but may come into existence sometime in future when new
  crypto cards are plugged.

  The most simple APQN entry comprises these two fields:
  + `adapter`: required, the CEX card number. Can be in the range of 0-255.
    Typically referred to as `adapter` number.
  + `domain`: required, the domain on the adapter. Can be in the range of 0-255.

  The tuple of these two numbers uniquely identifies an APQN within one hardware
  instance. If the compute nodes are distributed over more than one hardware
  instance, an extra entry is needed to distinguish an APQN(a,d) on hardware
  instance 1 from APQN(a,d) on hardware instance 2:

  + `machineid`: optional, is only required when the compute nodes are
    physically located on different hardware instances and the APQN pairs
    (adapter, domain) are not unique. If specified, the value must entered as
    follows:  
    `<manufacturer>-<machinetype>-<sequencecode>`
    with
    - `<manufacturer>` – value of the `Manufacturer` line from `/proc/sysinfo`
    - `<machinetype>` – value of the `Type` line from `/proc/sysinfo`
    - `<sequencecode>` – value of the `Sequence Code` line from `/proc/sysinfo`  
  
  For example, a valid value for `machineid` is `IBM-3906-00000000000829E7`.

  So the tuple (a,d) gets extended with the machineid which is unique per
  hardware instance and the triple (a,d,maschineid) now identifies an APQN again
  uniquely within the hardware instances. 
    
<!-- BEGIN - Commented out. This is stuff for future development

  Instead of the `adapter` field a `serialnr` field can be specified:

  + `serialnr`: specifies the serial number of the crypto card as listed in
    the respective sysfs file `/sys/devices/ap/cardxx/serialnr`.  
    For example, `93AABEET` is a valid serial number string. The serial number
    of a CEX crypto card is unique world-wide.
    A `domain` value is required to identify an APQN using (serialnr,domain).

    xxx an explanation of how to obtain the serial number for a card should
    be here also - TKE ? sysfs 
  
#### Alternative options to identify APQNs

Alternatively APQNs can be identified based on the CEX mode. You can use them 
instead of specifying `apqn`.

- `ccaaesmkvp`: specifies the CCA AES master key verification pattern against
  which all APQNs on all the compute nodes are matched. This hexadecimal value 
  is listed for each CCA queue `xx.yyyy` in the
  respective sysfs file `/sys/devices/ap/cardxx/xx.yyyy/mkvps` in the
   `AES CUR` line.  For example, `0xb072bc5c245aac8a` is a valid value.
  TODO for v1.1 !!!

- `ep11wkvp`: specifies the EP11 master wrapping key verification pattern
  against which all APQNs on all the compute nodes are matched. This hexadecimal value 
  is listed for each EP11 queue `xx.yyyy` in the
  respective sysfs file `/sys/devices/ap/cardxx/xx.yyyy/mkvps` in the `WK CUR`
  line. For example,
  `0xef490ddfce10b330b86cfe6db2ae2db98d65e8c19d9cb7a1b378dec93e398eb0` is a valid value.
  TODO for v1.1 !!!

END -->

<!-- RB: to be discussed
An APQN must not be member of more than one crypto config set.  In the absense of any other
parameters (like cexmode and mincexgen) that means an APQN identifier must not be a member
of more than one crypto config set. If an APQN identifier is a member of more than one crypto
config set, then each set must contain additional paramerters such that only one crypto 
config set declares a valid APQN with said APQN identifier (e.g. each crypto config set 
including APQN identifier (1,1) has a different cexmode).
Note, version 1 will enforce that no APQN identifier is a memeber of more than one crypto config set.
-->

#### Establishing the CEX Resource Configuration Map

In the end the CEX Resource Configuration Map is a kubernetes ConfigMap with
name `cex-resources-config` in the kubernetes namespace `kube-system`. If you
have this as a yaml file then a simple

    kubectl create -f <my_cex_resources.yaml>

deploys the clusterwide CEX ConfigMap. Updates and maintenance steps are
described in
[CEX configuration ConfigMap updates](#cex-configuration-configmap-updates).

### Installing and Configuring the CEX device plugin

#### Getting the CEX device plugin

The sources of the CEX device plugin are located on github
[https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin).

The CEX device plugin comprises 7 golang source files which are build into one static
binary which is embedded into a container image. There is a sample *Dockerfile*
provided in the git repository to build the go code and package the binary into
a container image.

Next the container image needs to pushed into the image repository of your
kubernetes cluster. This step highly depends on the actually cluster and it's
configuration and won't be described here.

#### Installing the CEX device plugin

The CEX device plugin image needs to be run on each worker node with administrator
privileges. Kubernetes knows the concept of a *DaemonSet* for this kind of
cluster wide service. The git repository shows a sample daemonset yaml file
which provides all the needed settings and options to run the CEX device plugin as
Kubernetes daemonset and a working sample is also given in appendix [Sample CEX
device plugin daemonset yaml](#sample-cex-device-plugin-daemonset-yaml).

Here are some important details for the daemonset needed to successfully run the
CEX device plugin:

- Namespace: The CEX device plugin instances need to run in namespace
  `kube-system` as this is the same namespace where the CEX ConfigMap
  resides.
- SecurityContext: The securitycontext needs to be *privileged*
  because the plugin code needs access to some directories and files on the
  worker node to establish a IPC connection to the kubelet, do some
  administrative tasks (create and destroy zcrypt additional device nodes) and
  build and provide some directory trees to be mounted into the client
  containers (shadow sysfs).
- Volumes: The plugin needs some volumes from the worker node:
  + `/dev` and `/sys` are needed to access the zcrypt device node and
    to add and remove *zcrypt additional device nodes* to be used by the
    crypto load containers.
  + The device-plugin API provided by Kubernetes is accessed via gRPC which
    needs the directory `/var/lib/kubelet/device-plugins`.
  + The CEX ConfigMap is accessed as a volume which provides one file
    `cex_resources.json` where the cluster wide CEX configuration is
    stored.
  + Access to `/var/tmp` is needed to build up the sysfs overlay
    directories for each container which uses crypto resources. More about the
    sysfs overlay see below under [The shadow sysfs](#the-shadow-sysfs).
- InitContainer: The commands needed related to deal with SELinux have been
  outsourced into an InitContainer. This InitContainer can be safely removed
  from the yaml file when there is no need.

After obtaining the CEX device plugin daemonset yaml file you should screen and maybe
update the plugin image source registry and then apply it with

    kubectl create -f <my_cex_plugin_daemonset.yaml>

and a few seconds later there should now run a pod 'cex-plugin' in namespace
'kube-system' on every worker node.

#### Some more details on the CEX device plugin

A CEX device plugin instance is an ordinary application build from golang code. The
application is very talkative and provides a lot of information about what is
going on via stdout/stderr. This output is available with the well known
`kubectl logs <pod>` command (don't forget the namespace `-n kube-system`
option).

The CEX device plugin application initially screens all the available APQNS on the
worker node, then reads in the CEX configuration. After verifying the CEX
configuration, for each config set a Kubernetes *device-plugin* with the
name of the config set is registered. So in the end there is one device plugin
registration per config set with full name
*cex.s390.ibm.com/\<config-set-name\>*.  
Details about Kubernetes device plugins can be found in
[\[6\]](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins)
and about the Device Plugin Manager in
[\[7\]](https://pkg.go.dev/github.com/kubevirt/device-plugin-manager/pkg/dpm).

After registration the CEX device plugin is ready for allocation requests forwarded
from the kubelet service. Such an allocation request is triggered by a crypto
load pod requesting a CEX resource from the config set. The allocation request
is processed which means to
- Create a new zcrypt device node and forward this to the container.
- Create sysfs shadow directories and make sure they are mounted onto the
  right place within the container.

Beside this main job there are some secondary tasks to do:
- APQN rescan: Every `APQN_CHECK_INTERVAL` (default is 30s) the
  available APQNs on the worker node are checked. When there are changes, the
  plugin reevaluates the list of available APQNs per config set and reannounces
  the list of plugin-devices to the Kubernetes system.
- CEX config map rescan: Every `CRYPTOCONFIG_CHECK_INTERVAL` (default
  is 120s) the crypto config map is reread. If the verification of the ConfigMap
  succeeds the changes will be reevaluated and eventually result in
  reannouncements to the Kubernetes system. If verification fails, an error
  message "Config Watcher: failed to verify new configuration!" is emitted and
  the plugin continues to run without CEX crypto configuration and is thus
  unable to satisfy allocation requests. This is explained in more detail at
  [CEX configuration ConfigMap updates](#cex-configuration-configmap-updates).
- Surveillance of pods with CEX resources allocated: Every
  `PODLISTER_POLL_INTERVAL` (default 30s) the list of pods which have an
  CEX resource assigned is examined. This is matched against the list of
  resources which are provided by the plugin. For each allocation request the
  plugin creates a zcrypt device node and some shadow sysfs directories. These
  resources need to get removed when no longer needed:
  + When the resources (zcrypt device node, shadow sysfs directories) which
    were created based on an allocation request are not used any more (the pod
    using the related plugin device has not been seen any more) for more than
    `RESOURCE_DELETE_UNUSED` (default 120s) seconds, these resources are
    destroyed.
  + When a zcrypt device node and the shadow sysfs directories which were
    created based on an allocation request have not been used (there was never
    seen a running pod with the related plugin device) for more than
    `RESOURCE_DELETE_NEVER_USED` (default 1800s) seconds, the zcrypt
    device node and the shadow sysfs directories are destroyed.


## Allocation of CEX resources by client containers

A container deployment can request one CEX resource from a CEX config set by
specifying a resource statement as part of the container specification.

    ...
    spec:
      containers:
      - image ...
        ...
        resources:
          limits:
            cex.s390.ibm.com/<config_set_name>: 1
        ...

For example a container requesting a CEX resource from the config set
`CCA_for_customer_1` from the sample ConfigMap in appendix
[Sample CEX resource Configuration Map](#sample-cex-resource-configuration-map)
needs this container specification:

    ...
    spec:
      containers:
      - image ...
        ...
        resources:
          limits:
            cex.s390.ibm.com/CCA_for_customer_1: 1
        ...

Appendix
[Sample CEX crypto load container](#sample-cex-crypto-load-container)
simple but complete sample yaml file for a customer load with CEX resource
allocation.

When the Kubernetes system tries to run an instance of this container it will
recognize the resource limitation. The CEX device plugin instances have hopefully
registered a bunch of plugin-devices for each of the config sets, so also some
plugin-devices for the `CCA_for_customer_1`. The Kubernetes system does the
bookkeeping for all these devices and therefore knows which devices are free and which
of the CEX device plugin instances had announced which devices. So the Kubernetes
system chooses one worker node where a CEX device plugin runs which had announced one
of the free plugin devices and forwards an allocation request to this plugin.

The plugin instance running on the worker node where the container will get
applied prepares the CEX resource and the sysfs shadow directories for the
container and returns these to the Kubernetes system and then the container is
started. The container will have a device node `/dev/z90crypt` customized to
have access to the allocated APQN and a customized `/sys/devices/ap` and
`/sys/bus/ap` providing a limited view of the AP/zcrypt world.

When the container finally finishes, the CEX device plugin on the worker node spots
this and cleans up the allocated resources - the customized additional zcrypt
device node and the sysfs shadow dirs - and the Kubernetes system marks the used
plugin-device as unused.

Here follow some more details as Q\&A paragraphs:

Q: What happens when all CEX resources within one config set are assigned to
running containers and a new pod/container requesting a CEX resource from this
config set is started?

A: Kubernetes will try to start the pod/container but the pod state is shown as
`pending`. A `kubectl describe` shows the reason:

    Warning  FailedScheduling  2m31s  default-scheduler  0/6 nodes are available: 1
      Insufficient cex.s390.ibm.com/<cex_config_set_name> ...

When finally a CEX resource from the config set becomes available, the `pending`
pod will get started automatically by the Kubernetes system.

Q: What happens when a CEX resource from a not existing or not defined CEX
config set is requested by a pod/container?

A: The Kubernetes cluster behaves similar to the out-of-CEX-resources within a
config set case. The pod is in `pending` state until finally a config set with
this name and a free CEX resource for this set comes into existence. Then the
CEX resource is assigned and the container started.

Q: I'd like to assign more than one APQN to the container to provide a backup
possibility for the running application. Is this supported?

A: As of now exactly one CEX resource may be requested by one container. The
idea for backups for cluster applications is to schedule more
pods/containers. This keeps the application within a container simple and easy
and delegates the backup and performance issues to the cluster system.

Q: I'd like to package an application into a container which uses different kinds
of CEX resources, for example one CCA and one EP11 APQN. So I'd like to assign
two APQNs from different config sets to one container. Does that work?

A: No. As of now exactly **one** CEX resource can be assigned to **one**
 container. This is only a limit to containers, but not to pods. As a pod may
 contain several containers each container may request one CEX resource from any
 config set. So split your application into units using only one type of CEX
 resource and package each unit into it's own container and so your pod load
 runs as multiple containers with each having it's own CEX resource.


## Details

### CEX configuration ConfigMap updates

From a cluster administration point of view it is desirable to change the CEX
configuration at the cluster wide Crypto config map for example to add or remove
CEX resources within a config set or even add or remove whole crypto config
sets.

This can be done during regular cluster uptime but with some carefulness. Every
`CRYPTOCONFIG_CHECK_INTERVAL` (default is 120s) the crypto config map is
re-read by all the CEX device plugin instances. The new config map is verified and if
valid then activated as the new current config map. On successful ConfigMap
re-read the plugin will log a message

    CryptoConfig: updated configuration

If the verification of the new CEX config map fails, the CEX device plugin will log an
error message. One reason for the verification failure may be the failure to
read or parse the ConfigMap resulting in error logs like

    CryptoConfig: Can't open config file ...

or

    CryptoConfig: Error parsing config file ...

If the verification step fails a message

    Config Watcher: failed to verify new configuration!

appears.

**All these failures result in running the plugin instances without any config map.**

The log messages appear periodically until yet another update of the config map
is finally accepted as valid.

Note that after an update of a config map, the cluster needs some time
(typically up to 2 minutes) to propagate the changes to all nodes.
Another (potentially faster) way to update the config map for the plugin is to
restart the rollout of the deployment via

    kubectl rollout restart daemonset <name-of-the-cex-plugin-daemonset> -n kube-system

This triggers a restart of each instance of the daemonset in a coordinated way
by Kubernetes.


### Over-commitment of CEX resources

By default a CEX resource (an APQN) maps to exactly one Kubernetes
*plugin-device*. This is the administration unit known by Kubernetes and in fact
a container is requesting such a plugin-device.

By default the CEX device plugin maps each available APQN to one plugin-device and so
in the end one APQN is assigned to a container requesting a CEX resource.

The CEX device plugin can provide more than one plugin-devices per APQN, which allows
some over-commitment of the available CEX resources.

Setting the environment variable `APQN_OVERCOMMIT_LIMIT` to a value greater than
1 (default is 1) allows to control how many plugin-devices are announced to the
Kubernetes system for each APQN. So for example with 3 APQNs available within a
config set and an over-commit value of 10 there are 30 CEX plugin-devices
allocatable and up to 30 containers could successfully request a CEX resource.

In the end more than one container will share one APQN with over-commitment
enabled. This exposes no security weakness, but may result in fewer
performance for the crypto operations within each container.


### The device node z90crypt

On a worker node the device node `/dev/z90crypt` offers access to all zcrypt
devices known to the worker running as a KVM guest. The application of a
container which requests a CEX resource will also see and use the device node
`/dev/z90crypt`. However, what's visible inside the container is in fact a newly
constructed z90crypt device with limited access to only the APQN assigned.

On the worker node these constructed z90crypt devices are visible in the
`/dev` directory as device nodes
`zcrypt-apqn-<card>-<domain>-<overcommitnr>`. With the start of the container
the associated device node on the worker node is mapped to the `/dev/z90crypt`
device inside the container.

These constructed z90crypt devices are created on the fly with the CEX
allocation request triggered with the container start and deleted automatically
when the container terminates.

With version 1 of the CEX device plugin the constructed zcrypt device nodes limit
access to exact one APQN (adapter, usage domain, no control domain), allowing
all ioctls. Please note that these settings allow not only usage but also
control actions restricted to the underlying APQN with the `/dev/z90crypt`
device visible inside the container, even with overcommited plugin-devices.


### The shadow sysfs

The CEX device plugin manipulates the AP part of the sysfs a container is able to
explore. The sysfs tree within a container contains two directories related to
the AP/zcrypt functionality: `/sys/bus/ap` and `/sys/devices/ap`.

Tools working with zcrypt devices (like `lszcrypt` or `ivp.e`) need to see the
restricted world which is accessible via the `/dev/z90crypt` device node within
the container.

The CEX device plugin creates a *shadow sysfs* directory tree for each of these
paths on the worker node at `/var/tmp/shadowsysfs/<plugin-device>`. With
the start of the container both directories `/sys/bus/ap` and
`/sys/devices/ap` are overlayed (overmounted) with the corresponding shadow
directory on the worker node.

These shadow directory trees are simple static files created from the original
sysfs entries on the worker node. So they loose their sysfs functionality and
show a static view of a limited AP/zcrypt world. For example the
`/sys/bus/ap/ap_adapter_mask` is a 256 bit field listing all available adapters
(crypto cards). The manipulated file appearing inside the container only shows
the adapter belonging to the assigned APQN. All load and counter values in the
corresponding sysfs attributes (for example
`/sys/devices/ap/card<xx>/<xx>.<yyyy>/request_count`) show up as 0 and don't get
updates when a crypto load is running.

This restricted sysfs within a container should be sufficient to satisfy the
discovery tasks of most applications (`lszcrypt`, `ivp.e`, opencryptoki with CCA
or EP11 token) but has limits. `chzcrypt` will fail to change sysfs attributes,
offline switch of a queue will not work and applications inspecting counter
values may get confused.

An administrator logged into a Kubernetes worker node could figure out the
assignment of a CEX resource and a requesting container (for example by reading
the log messages from the plugins). Without over-commitment the counters of an
APQN on the worker node reflect the crypto load of the associated container and
`lszcrypt` can be used.


### Hot-Plug and Hot-Unplug of APQNs

The CEX device plugin monitors the APQNs available on the worker node by default every
30 seconds. This comprises the existence of APQNs and their *online* state. When
the worker node runs as a KVM guest it is possible to *live* modify the devices
section of the guest's xml definition at the KVM host resulting in APQNs
appearing or disappearing. The AP bus and zcrypt device driver inside the Linux
system recognizes this as hot plug or unplug of crypto cards and/or domains.

It is also possible to directly change the *online* state of a card or APQN
within a worker node. So an APQN may be available but switched to *offline*
by intention by an system administrator.

There exists a dialog on the HMC which makes it possible to *configure off* and
*configure on* CEX cards assigned to an LPAR. A CEX card in *config off* is
still visible in the LPAR and thus in the worker node but similar to the
*offline* state no longer usable.

All this might cause the CEX device plugin to deal with varying CEX
resources. The plugin code is capable of handling hotplug and hotunplug and the
*online* state changes of CEX resources and reports changes in the config set to
the Kubernetes system. Because of this handling it is possible to include APQNs
into the CEX config sets which may not exist at the time of first deployment of
the CEX config map. At a later time the card is hot-plugged and assigned to the
running LPAR and the cluster will spot this and make the appearing APQNs, which
are already a member in a config set, available for allocation requests.

The handling of the *online* state is done by reporting the relevant plugin
devices as *healthy* (*online*) or *unhealthy* (*offline*). An *unhealthy*
plugin device is not considered when a CEX resource allocation takes place.

Remark: It may happen that a CEX resource becomes unusable (hot unplug or
offline state) but is assigned to a running container. The plugin recognizes the
state change, updates all the bookkeeping stuff and reports this to the
Kubernetes system but does **not** stop or kill the running container. It is
assumed that the container load will fail anyway as the AP bus or zcrypt device
driver on the worker node will react with failures on the attempt to use such a
CEX resource device. A well designed cluster application will terminate with a
bad return code causing Kubernetes to re-establish a new container which will
claim a CEX resource and the situation recovers automatically.


###  SELinux and the Init Container

The CEX device plugin prepares various files and directories that become mounted
to the pod at an allocation request.Among those mounts are the directories
descibed under [The shadow sysfs](#the-shadow-sysfs). These folders are
generated on the worker node and mounted into the new pod. In some cases,
special actions are needed for such a mount to be accessible inside the newly
created pod. One example is SELinux where the folder (or one of its parent
folders) needs to have the appropriate SELinux label. Other security mechanisms
might have different requirements.

Because the security mechanisms and their configuration depend on the
cluster instance, the CEX device plugin does not provide any support
for such mechanisms.  Instead, in the SELinux case, an Init Container
can be used to set the correct label on the shadow sysfs root folder
`/var/tmp/shadowsysfs` which will contain all the sub-folders that are
mapped into pods.  See [Sample CEX device plugin daemonset
yaml](#sample-cex-device-plugin-daemonset-yaml) for an example of a daemonset
deployment of the CEX device plugin that contains an init container to
set up `/var/tmp/shadowsysfs` for use in a SELinux-enabled environment.

### Limitations

#### Namespaces and the project field

The `project` field of a CEX config set should match to the namespace of the
container requesting an member of this set. So only *blue* applications should
be able to allocate *blue* APQNs from the *blue* config set.

Unfortunately the allocation request forwarded from the Kubernetes system to the
CEX device plugin does not provide any namespace information. So the plugin is not able
to check the namespace affiliation.

When the container runs, the surveillance loop of the CEX device plugin will detect
this mismatch and emit a log entry  
`PodLister: Container <aaa> in namespace <bbb> uses a CEX resource <ccc> marked for project <ddd>!`.

This behavior may be a security risk as this opens the possibility to use the
HSM of another group of applications. However, to really exploit this more is
needed for example a secure key from the target to attack or the possibility to
insert a self made secure key into the target application.

There is a workaround for this. By setting quotas for all namespaces but the one
which is allowed to use the resource. Here is an example:

    apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: cex-blue-quota-no-red
      namespace: blue
    spec:
      hard:
        requests.cex.s390.ibm.com/red: 0
        limits.cex.s390.ibm.com/red: 0

This yaml snippet restricts the namespace *blue* to allocate zero CEX resources
from the crypto config set *cex.s390.ibm.com/red*. So effectively all containers
belonging to the *blue* namespace can't allocate *red* CEX resources any more.

At appendix [Sample CEX quota restriction
script](sample-cex-quota-restriction-script) a bash script which produces
a yaml file which establishes these quota restrictions can be found.


## Appendix

### Sample CEX resource configuration map

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

### Sample CEX device plugin daemonset yaml

    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
      name: cex-plugin-daemonset
      namespace: kube-system
    spec:
      selector:
        matchLabels:
          name: cex-plugin
      template:
        metadata:
          annotations:
            scheduler.alpha.kubernetes.io/critical-pod: ""
          labels:
            name: cex-plugin
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
            name: cex-plugin
            securityContext:
              privileged: true
            env:
              # provide NODENAME to the container
              - name: NODENAME
                valueFrom:
                  fieldRef:
                    fieldPath: spec.nodeName
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

### Sample CEX crypto load container

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

### Sample CEX quota restriction script

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


## References

[1] IBM z15 Configuration Setup  
[https://www.redbooks.ibm.com/abstracts/sg248860.html](https://www.redbooks.ibm.com/abstracts/sg248860.html)

[2] Linux on Z and LinuxONE  
[https://www.ibm.com/docs/en/linux-on-systems?topic=linux-z-linuxone](https://www.ibm.com/docs/en/linux-on-systems?topic=linux-z-linuxone)

[3] CryptoCards  
[https://www.ibm.com/security/cryptocards](https://www.ibm.com/security/cryptocards)

[4] System z Crypto and TKE Update  
[https://www.redbooks.ibm.com/abstracts/sg247848.html](https://www.redbooks.ibm.com/abstracts/sg247848.html)

[5] CCA - Common Cryptographic Architecture functional overview  
[https://www.ibm.com/docs/en/linux-on-systems?topic=cca-overview](https://www.ibm.com/docs/en/linux-on-systems?topic=cca-overview)

[6] kubernetes Device Plugins  
[https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins)

[7] Device Plugin Manager  
[https://pkg.go.dev/github.com/kubevirt/device-plugin-manager/pkg/dpm](https://pkg.go.dev/github.com/kubevirt/device-plugin-manager/pkg/dpm)


## Notices

This information was developed for products and services offered in the U.S.A. This material might be available from IBM® in other languages. However, you may be required to own a copy of the product or product version in that language in order to access it.

IBM may not offer the products, services, or features discussed in this document in other countries. Consult your local IBM representative for information on the products and services currently available in your area. Any reference to an IBM product, program, or service is not intended to state or imply that only that IBM product, program, or service may be used. Any functionally equivalent product, program, or service that does not infringe any IBM intellectual property rights may be used instead. However, it is the user's responsibility to evaluate and verify the operation of any non-IBM product, program, or service.

IBM may have patents or pending patent applications covering subject matter described in this document. The furnishing of this document does not give you any license to these patents. You can send license inquiries, in writing, to:

*  IBM Director of Licensing   
     IBM Corporation   
     North Castle Drive, MD-NC119   
     Armonk, NY 10504-1785   
     United States of America*

For license inquiries regarding double-byte \(DBCS\) information, contact the IBM Intellectual Property Department in your country or send inquiries, in writing, to:

*Intellectual Property Licensing   
 Legal and Intellectual Property Law   
 IBM Japan Ltd.   
 19-21, Nihonbashi-Hakozakicho, Chuo-ku   
 Tokyo 103-8510, Japan*

INTERNATIONAL BUSINESS MACHINES CORPORATION PROVIDES THIS PUBLICATION “AS IS” WITHOUT WARRANTY OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF NON-INFRINGEMENT, MERCHANTABILITY, OR FITNESS FOR A PARTICULAR PURPOSE. Some jurisdictions do not allow disclaimer of express or implied warranties in certain transactions, therefore this statement may not apply to you.

This information could include technical inaccuracies or typographical errors. Changes are periodically made to the information herein; these changes will be incorporated in new editions of the publication. IBM may make improvements and/or changes in the product\(s\) and/or the program\(s\) described in this publication at any time without notice.

Any references in this information to non-IBM websites are provided for convenience only and do not in any manner serve as an endorsement of those websites. The materials at those websites are not part of the materials for this IBM product and use of those websites is at your own risk.

IBM may use or distribute any of the information you supply in any way it believes appropriate without incurring any obligation to you.

Licensees of this program who want to have information about it for the purpose of enabling: \(i\) the exchange of information between independently created programs and other programs \(including this one\) and \(ii\) the mutual use of the information which has been exchanged, should contact

*  IBM Director of Licensing   
     IBM Corporation   
     North Castle Drive, MD-NC119   
     Armonk, NY 10504-1785   
     United States of America*

Such information may be available, subject to appropriate terms and conditions, including in some cases, payment of a fee.

The licensed program described in this document and all licensed material available for it are provided by IBM under terms of the IBM Customer Agreement, IBM International Programming License Agreement, or any equivalent agreement between us.

Information concerning non-IBM products was obtained from the suppliers of those products, their published announcements or other publicly available sources. IBM has not tested those products and cannot confirm the accuracy of performance, compatibility or any other claims related to non-IBM products. Questions on the capabilities of non-IBM products should be addressed to the suppliers of those products.

This information is for planning purposes only. The information herein is subject to change before the products described become available.

This information contains examples of data and reports used in daily business operations. To illustrate them as completely as possible, the examples include the names of individuals, companies, brands, and products. All of these names are fictitious and any similarity to actual people or business enterprises is entirely coincidental.

Portions of this information are provided under the Apache v2 license [https://www.apache.org/licenses/LICENSE-2.0](https://www.apache.org/licenses/LICENSE-2.0).

COPYRIGHT LICENSE:

This information contains sample application programs in source language, which illustrate programming techniques on various operating platforms. You may copy, modify, and distribute these sample programs in any form without payment to IBM, for the purposes of developing, using, marketing or distributing application programs conforming to the application programming interface for the operating platform for which the sample programs are written. These examples have not been thoroughly tested under all conditions. IBM, therefore, cannot guarantee or imply reliability, serviceability, or function of these programs. The sample programs are provided "AS IS", without warranty of any kind. IBM shall not be liable for any damages arising out of your use of the sample programs.

### Trademarks

Helm™ and the Helm logo are trademarks of The Linux® Foundation, and use of them as a trademark is subject to The Linux Foundation's Trademark Usage Guidelines at [https://www.linuxfoundation.org/trademark-usage/](https://www.linuxfoundation.org/trademark-usage/).

IBM, the IBM logo, and ibm.com® are trademarks or registered trademarks of International Business Machines Corp., registered in many jurisdictions worldwide. Other product and service names might be trademarks of IBM or other companies. A current list of IBM trademarks is available on the Web at [Copyright and trademark information](https://www.ibm.com/legal/copytrade) at www.ibm.com/legal/copytrade.shtml.

Java™ and all Java-based trademarks and logos are trademarks or registered trademarks of Oracle and/or its affiliates.

The registered trademark Linux is used pursuant to a sublicense from the Linux Foundation, the exclusive licensee of Linus Torvalds, owner of the mark on a worldwide basis.

Kubernetes® and the Kubernetes logo are registered trademarks of The Linux Foundation, and use of them as a trademark is subject to The Linux Foundation's Trademark Usage Guidelines at [https://www.linuxfoundation.org/trademark-usage/](https://www.linuxfoundation.org/trademark-usage/).

Red Hat®, OpenShift®, and Ansible® are registered trademarks of Red Hat, Inc. or its subsidiaries in the United States and other countries.

UNIX® is a registered trademark of The Open Group in the United States and other countries.

### Terms and conditions for product documentation

Permissions for the use of these publications are granted subject to the following terms and conditions.

-   **Applicability**

    These terms and conditions are in addition to any terms of use for the IBM® website.

-   **Personal use**

    You may reproduce these publications for your personal, noncommercial use provided that all proprietary notices are preserved. You may not distribute, display or make derivative work of these publications, or any portion thereof, without the express consent of IBM®.

-   **Commercial use**

    You may reproduce, distribute and display these publications solely within your enterprise provided that all proprietary notices are preserved. You may not make derivative works of these publications, or reproduce, distribute or display these publications or any portion thereof outside your enterprise, without the express consent of IBM®.

-   **Rights**

    Except as expressly granted in this permission, no other permissions, licenses or rights are granted, either express or implied, to the publications or any information, data, software or other intellectual property contained therein.

    IBM® reserves the right to withdraw the permissions granted herein whenever, in its discretion, the use of the publications is detrimental to its interest or, as determined by IBM®, the above instructions are not being properly followed.

    You may not download, export or re-export this information except in full compliance with all applicable laws and regulations, including all United States export laws and regulations.

    IBM® MAKES NO GUARANTEE ABOUT THE CONTENT OF THESE PUBLICATIONS. THE PUBLICATIONS ARE PROVIDED "AS-IS" AND WITHOUT WARRANTY OF ANY KIND, EITHER EXPRESSED OR IMPLIED, INCLUDING BUT NOT LIMITED TO IMPLIED WARRANTIES OF MERCHANTABILITY, NON-INFRINGEMENT, AND FITNESS FOR A PARTICULAR PURPOSE.
