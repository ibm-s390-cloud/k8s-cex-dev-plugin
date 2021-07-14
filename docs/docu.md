# Kubernetes Device Plugin for IBM CryptoExpress (CEX) cards

This document describes version 1 of the Kubernetes Device Plugin for IBM
CryptoExpress (CEX) cards (s390 / IBM Z & LinuxONE).

Authors:

- Harald Freudenberger <freude@linux.ibm.com>
- Juergen Christ <jchrist@linux.ibm.com>

Changelog:

- 2021/07/08: first draft version

## Overview

This Kubernetes device plugin provides access to IBM s390 Crypto Express (CEX)
cards for s390 Kubernetes container loads.  Below the term CEX device plugin
refers to this Kubernetes device plugin.

A CEX config map defines one or more crypto configuration sets. A crypto
configuration set consists of resources of one or more IBM CryptoExpress
cards. A set is identified by its name and is instantiated as a Kubernetes
extended resource. Each crypto configuration set provides a pool of equivalent
CEX crypto resources. Containers can request one item by requesting such an
Kubernetes extended resource.

The CEX device plugin instances running on all compute nodes handle allocation
requests, check if the existing crypto resources are available on the nodes,
claim the resource and ensure containers are scheduled on the right nodes with
the requested CEX crypto resources.

The CEX device plugin handles:
- Bookkeeping of the CEX crypto resources
- Monitoring of all containers with allocated CEX crypto resources
- Cleaning up after container runtimes ended
- Hot pluging (and unplugging) of CEX crypto resources

## Setting up the environment for the CEX device plugin

Before setting up the CEX Device Plugin, the CEX crypto resources on the
compute nodes of the cluster might need to be set up. This is not specific
to the use of the CEX device plugin and, thus, below a brief summary with
references to existing documentation.

### Overview on hardware, CEX crypto resources, and Kubernetes compute nodes

xxx CEX crypto cards, modes of operation, domain partitioning, HSM and APQNs
CEX card Serialnumber, machine id

### Assignment of CEX crypto resources to LPARs, KVM, and z/VM

xxx LPAR crypto resources customized on the HMC (see docu xxx)

xxx KVM guest crypro resources done on the KVM host (see KVM docu xxx)

xxx z/VM guest crypto resources xxx (see docu xxx)

### Distributing CEX crypto resources for compute nodes across physical
and/or virtual machine boundaries

xxx Distributed cluster compute nodes across several physical and/or virtual machines
and APQN address clashing

xxx APQN address clashing

### CEX crypto resources

xxx What comes here???

## Installing the CEX device plugin

xxx TODO Provide more details on how to install the plugin... will be more
    easiler once the container image is out

Document here:
- daemonset

## Configuring for the CEX device plugin

The configuration for CEX crypto resources the CEX device plugin handles
is defined in a configmap using the JSON format.  The configuration defines
one ore more unique crypto _configuration sets_. Each set groups CEX crypto
resources consisting of equivalent APQNs.

### Considerations for equally configured APQNs

Within each configuration set all the APQNs have to be set up in an equal
way. For each CEX mode, consider:
- CCA CEX resources require that the Master Keys and Access Control Points
  must be equal.
- EP11 CEX resources require an equal Master Wrapping Key and Control
  Settings.
- CEX accelerator resources are stateless and do not need any equal setup.

A container must request exactly **one** configuration set and obtains **one**
CEX crypto resource by the CEX device plugin if one APQN is available,
healthy, and not already allocated.  The APQN is randomly choosen and will be
assigned to the life time of the container.

#### Sample flow on how an APQN is assigned to a container applications

The cluster administrator defines a configuation set named *blue* with as
set of APQNS and a set named *red* of APQNS. This differentiation might reflect
different tenants (*blue* and *red*) with dedicated CEX crypto resources for
each of them. The *blue* configuration set is for use of
applications (containers) from tenant *blue*, the *red* configuration set is
for use of application by tenant *red*.  Each APQN within the *blue* and *red*
configuration set is assumed to be equal. That means all application of *blue*
can perform the correct crypto operations with any of the APQNs from the *blue*
configuration set. The same applies to application containers using APQNs from
the *red* configuration set.

At start of a container requesting the *blue* configuration set, the CEX
device plugin gets an allocation request for an *APQN* of the *blue*
configuration set.  The CEX device plugin chooses one random APQN which is
available, healthy, and not occupied.  The APQN is then assigned to the
container.  Because the assigned APQN of the *blue* configuration set is
provided by exactly one compute node, it is determined where exactly the
container can run.  Further, this decision is made internally by the
interaction of the CEX device plugin and the Kubernetes infrastructure
services.

### Creating configuration sets of CEX crypto resources

The clusterwide configuration of the CEX crypto resources crypto is kept
in a Kubernetes ConfigMap within the `kube-system` namespace.  The ConfigMap
must be named `cex-resources-config`.  The content of the ConfigMap is a
config file section in JSON format and looks like this:

````json
    {
        "cryptoconfigsets":
        [
            {
                "setname":   "blue",
                "project":   "blue_project",
                "apqns":
                [
                    {
                        "adapter":    1,
                        "domain":     6,
                    },
                    {
                        "adapter":    7,
                        "domain":     6,
                    },
                ]
            },
            {
                "setname":   "red",
                "project":   "red_project",
                "apqns":
                [
                    {
                        "adapter":    1,
                        "domain":    11,
                    },
                    {
                        "adapter":    7,
                        "domain":    11,
                    },
                ]
            }
        ]
    }
````

The configmap defines a list of configuration sets. Each configuration set
comprises these entries:

#### Basic definitions

- `setname`: required, might be any string value, needs to be unique within all
  the configuration sets. This is the identifier used by the container to request
  one of the CEX crypto resources from within this set.

- `project`: required, namespace of this configuration set. Only containers with
  matching namespace can access CEX crypto resources of this set.
  Currently this is **not implemented** due to missing namespace information
  at allocation time. This might change with future Kubernetes versions.

- `cexmode`: optional, specifies the required CEX mode if specified and must be
  one of `ep11`, `cca`, or `accel`. Enables an additional check step every time
  the APQNs on each node are screened by the CEX device plugin: all APQNs
  of this configuration set must match the specified CEX mode. On mismatches,
  the CEX device plugin creates an log entry and discards the use of this
  APQN for the set.
  TODO for v1.0 !!!

- `mincexgen`: optional, specifies the minimal CEX card generation for this
  configuation set. If specified, must match to `cex[4-7]`. Enables an
  additional check step every time the APQNs on each compute node are screened:
  all APQNs of this configuration set are checked to have at least the
  specified CEX card generation. On mismatches, the CEX device plugin creates
  an log entry and discards the use of the APQN for this configuration set.
  TODO for v1.0 !!!

#### Defining APQNs

- `apqns`: A list of APQN entries is listed here. The list might be empty (and
  exist only for future availability) but should typically include at least one
  APQN entry. The specified list is assumed to be a list of equivalent APQNs
  (see also [Considerations for equally configured
  APQNs](#considerations-for-equally-configured-apqns).

  The most simple APQN entry might comprise these two fields:
  + `adapter`: required, the CEX card number in the range of 0-255,
    typically referred to as `adapter` number.
  + `domain`: required, the domain on the adapter to use the range of 0-255.

  The tuple of these two numbers is uniquely identifying an APQN within one
  hardware instance. If the compute nodes are distributed over more than one
  hardware intance, an additional field might be needed to distinguish an
  APQN(a,d) on hardware instance 1 from APQN(a,d) on hardware instance 2:

  + `machineid`: optional, is only required when the compute nodes are physically
  located on different hardware instances and the APQN pairs (adapter, domain)
  are not unique. If specified, the value must be in format
  `<manufacturer>-<machinetype>-<sequencecode>`
  with
  `<manufacturer>` – value of the `Manufacturer` line from `/proc/sysinfo`
  `<machinetype>` – value of the `Type` line from `/proc/sysinfo`
  `<sequencecode>` – value of the `Sequence Code` line from `/proc/sysinfo`
  For example, a valid value for `machineid` might be
  `IBM-3906-00000000000829E7`.

  Instead of the `adapter` field a `serialnr` field can be specified:

  + `serialnr`: specifies the serial number of the crypto card as listed in
  the respective sysfs file `/sys/devices/ap/cardxx/serialnr`.
  For example, `93AABEET` is a valid serial number string. The serial number
  of a CEX crypto card is world-wide unique.  A `domain` value is required
  to identify an APQN using (serialnr,domain).
  TODO for v1.0 !!!

#### Alternative options to identify APQNs

There are an alternative to identify APQNs based on the CEX mode. Use those
instead of specifying `apqn`.

- `ccaaesmkvp`: specifies the CCA AES Master Key Verification Pattern against
  which all APQNs on all the compute nodes are matched. This is the
  hexadecimal value and is listed for each CCA queue `xx.yyyy` in the
  respective sysfs file `/sys/devices/ap/cardxx/xx.yyyy/mkvps` in the `AES
  CUR` line.  For example, `0xb072bc5c245aac8a` could be a valid value.
  TODO for v1.0 !!!

- `ep11wkvp`: specifies the EP11 Master Wrapping Key Verification Pattern
  against which all APQNs on all the compute nodes are matched. This is the
  hexadecimal value and is listed for each EP11 queue `xx.yyyy` in the
  respective sysfs file `/sys/devices/ap/cardxx/xx.yyyy/mkvps` in the `WK CUR`
  line. For example,
  `0xef490ddfce10b330b86cfe6db2ae2db98d65e8c19d9cb7a1b378dec93e398eb0` could
  be a valid value.
  TODO for v1.0 !!!

### Further considerations

For high availability and scaling of container applications using crypto
configuration sets, distribute equally configured APQNs widely across all
the compute nodes in the Kubernetes cluster.

The CEX device plugn is hotplugging-aware. You can list APQNs that are not
(yet) available but might become later alive.  For details, see Details
section below.
TODO: add reference to details section

For a sample config map with CEX crypto configuration sets can be found
in the appendix.

The clusterwide CEX resource config map is provided to the CEX device plugin
daemonset as a volume which shadows the crypto config sets into a config file
`/work/cex_resources.json`. This config file is read only once every CEX device
plugin daemonset instance start. So changes on the clusterwide CEX resource
config map are not pushed and evaluated by already running plugin
instances. More details and solutions can be found in the details chapter.

## Using the CEX device plugin

### Requesting crypto configuration sets for container applications

Each container within a pod definition might request exactly **one** APQN
from **one** of the defined crypto configuration sets. Use the `resources`
statement in the image declaration of the containers section.
The CEX device plugin advertises the crypto configuration sets using the
`cex.s390.ibm.com/` prefix.

````yaml
    spec:
      containers:
      - image: 'container-image-using-cex'
        imagePullPolicy: Always
        name: whatever
        ...
        resources:
          limits:
            cex.s390.ibm.com/<name_of_the_cex_crypto_config_set>: 1
        ...
````

The Kubernetes infrastructure and the CEX device plugin performs these steps:

1. Check the currently available and healthy APQNs of the specified crypto
   configuration set.
2. Randomly choose one free APQN from the configuration set.  If all APQNs
   are occupied, starting the container will be delayed until an APQN becomes
   available.
3. The CEX device plugin instance on the compute node where the APQN from
   step 2 is located will claim the APQN and prepares it for use.
   (Preparation includes creating a logical CEX device node, prepare shadow
   sysfs directories, and perform book keeping of its assignment.)
4. The container becomes started on the compute node where the APQN from step
   2 is located. Within the container namespace, there will be the
   `/dev/z90crypt` device.  Operations on this device are restricted to the
   assigned APQN only on the compute node. Further, in the sysfs for the
   container namespace, only the assigned APQN is visible in the sysfs
   subtrees `/sys/devices/ap` and `/sys/bus/ap`.

The CEX device plugin performs bookkeeping by monitoring of APQNs during the
runtime of containers.

After the container ended, the CEX device plugin instance on the compute node
will clean up the resources created in the preparation step. (Destroying the
logical CEX device node, remove shadow sysfs directories, and bookkeeping
data.)

Note that one crypto configuration set can be allocated per container.
Decompose applications requiring two or more crypto configuration sets into
multiple containers.  Consider to deploy those containers in the same POD
to share namespaces.

A sample container application requesting a crypto configuration set can be
found in the appendix.


Overcommitment ...

## Details and technical reference

### config map details

#### optional fields

#### Updates and Rolling out changes on the clusterwide CEX resource configuration

on the  out changes on the clusterwide how to roll out changes on the clusterwide config map

### CEX Device Plugin Details

#### config map

static config map only read once at CEX plugin container start.

`kubectl rollout restart deployment <cex-plugin-daemonset-deployment>` to
restart the running CEX plugin daemonset instances and so re-read and activate
the changes on the clusterwide CEX crypto config map.

#### CEX resource registration

Registration domain is `cex.s390.ibm.com`

#### APQN detection, hotplug and hotunplug, healthy state

#### The CEX device plugin application

#### The shadow sysfs

#### podlister

#### Ccustomization with environment variables

Overcommitment of CEX crypto resources

#### Selinux and the Init Container

#### All Volume Mounts required by the CEX Device Plugin


### Appendix

#### Sample CEX resource configuration map

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
                "setname":   "blue",
                "project":   "blue_project",
                "cextype":   "cca",
                "apqns":
                [
                    {
                        "adapter":    1,
                        "domain":     6,
                    },
                    {
                        "adapter":    2,
                        "domain":     6,
                    }
                ]
            },
            {
                "setname":   "red",
                "project":   "red_project",
                "cextype":   "ep11",
                "apqns":
                [
                    {
                        "adapter":    1,
                        "domain":    11,
                        "machineid": "IBM-3906-00000000000829E7"
                    },
                    {
                        "adapter":    1,
                        "domain":    11,
                        "machineid": "IBM-2964-000000000007EC87"
                    }
                ]
            },
            {
                "setname":    "green",
                "project":    "green_project",
                "cextype":    "cca",
                "ccaaesmkvp": "0xb072bc5c245aac8a"
            },
            {
                "setname":   "yellow",
                "project":   "yellow_project",
                "ep11wkvp":
            }
        ]
    }

#### Sample crypto load container

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: testload-for-blue-project
      namespace: default
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: testload-for-blue-project
      strategy:
        type: Recreate
      template:
        metadata:
          labels:
            app: testload-for-blue-project
        spec:
          containers:
          - image: 'bash'
            imagePullPolicy: Always
            name: testload-for-blue-project
            command: ["/bin/sh", "-c", "while true; do echo test-crypto-load; sleep 30; done"]
            resources:
              limits:
                cex.s390.ibm.com/blue: 1

#### Sample plugin deployment


####
control domain
