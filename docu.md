# Kubernetes Device Plugin for IBM CryptoExpress (CEX) cards

This document documents version 1 of the s390 Kubernetes Device Plugin for IBM
CryptoExpress (CEX) cards.

Authors:

- Harald Freudenberger <freude@linux.ibm.com>
- Juergen Christ <jchrist@linux.ibm.com>

Changelog:

- 2021/07/08: first draft version

## Overview

The kubernetes CEX plugin provides access to IBM s390 Crypto Express (CEX) cards
for s390 kubernetes container loads.

A CEX config map defines one or more crypto configuration sets. A crypto
configuration set consists of resources of one or more IBM CryptoExpress
cards. A set is identified by its name and is made available as Kubernetes
extended resource. Each crypto config set provides a pool of equivalent CEX
crypto resources and containers can request one item by requesting such an
extended kubernetes resource.

The CEX device plugin instances running on all the worker nodes handle the
allocation requests, check with the existing crypto resources available on the
different nodes, claim the resource and make sure the container is scheduled on
the right node with the reserved CEX resource.

Bookkeeping of the CEX resources, monitoring of all containers with allocated
CEX resources and proper cleanup after container runtime is handled by the CEX
plugin as well as hot plug (and unplug) of CEX resources.

## Setting up and using the Kubernetes CEX Device Plugin

Before actually setting up the CEX Device Plugin, the CEX resources on the
worker nodes of the cluster may need to be setup. In general this is a step not
specific to the use of the CEX device plugin. So here is only a brief summary of
how to set up CEX resources for worker nodes.

### S390 hardware, CEX resources and kubernetes worker nodes

xxx CEX crypto cards, modes of operation, domain partitioning, HSM and APQNs
CEX card Serialnumber, machine id

### Assignment of CEX resources to s390 LPARs, KVM and zVM worker nodes

xxx LPAR crypto resources customized on the HMC (see docu xxx)

xxx KVM guest crypro resources done on the KVM host (see KVM docu xxx)

xxx zVM guest crypto resources xxx (see docu xxx)

xxx Distributed cluster worker nodes across several physical and/or virtual machines
and APQN address clashing.

### s390 CEX resources

### The CEX resource configuration map

The CEX resource configuration map is a yaml config map file which groups the
CEX resources available in the cluster into unique sets of equvialent
APQNs. Within each CEX config set all the APQNs are to be set up in a equal
way. For CCA CEX resources this means to have equal Master Keys and equal Access
Control Points; EP11 CEX resources require to have an equal Master Wrapping Key
and equal Controll Settings; CEX Accelerator resources are stateless and don't
need any equal setup.

A container is requesting **one** CEX resource from exactly **one** CEX config
set and the CEX Plugin will randomly choose any one APQN which is available,
healthy and not already allocated and assign it to the container for it's
livetime.

An example may illustrate this: The cluster administrator defines in the CEX
resource configuration map a *blue* set of APQNS and a *red* set of APQNS. This
differentiation may reflect the seperation of two companies (*blue* and *red*)
with dedicated CEX resources for each company. The *blue* set is for use of
applications (containers) from the *blue* company, the *red* set is for use of
*red* applications. Each APQN within the *blue* and the *red* set is assumet to
be equal in the sense of a *blue* application can do the correct crypto
operations with anyone of the APQNs of the *blue* set. The same is with the
*red* containers and the members of the *red* cex config set.

With the start of a *blue* container the CEX Plugin gets an allocation request
for a *blue* APQN and chooses one random APQN within the *blue* set which is
available, healty and not occupied and assigns it to the container. As the
chosen APQN is provided by exactly one worker node, it is determined where
exactly this *blue* container comes to run. However, this decision is made under
the hood by the CEX plugin instances in interaction with the kubernetes
infrastructure services.

The clusterwide CEX resources crypto configuration is kept in an kubernetes
ConfigMap with namespace `kube-system`. The content of the ConfigMap is a config
file section in JSON format and looks like this:

    {
        "cryptoconfigsets":
        [
            {
                "setname":   "blue",
                "project":   "blue_customer",
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
                    ...
                ]
            },
            {
                "setname":   "red,
                "project":   "red_customer",
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
                    ...
                ]
            },
            {
                "setname": ...
                ...
            }
        ]
    }

It is a list of CEX config sets where each config set comprises these entries:
+ `setname`: required, may be any string value, needs to be unique within all
  the CEX config sets. This is the identifier used by the container to request
  one of the CEX resources within this set.
+ `project`: required, namespace of this CEX config set. Only containers with
  matching namespace can access the crypto resources of this set. As of now this
  is **not implemented** as there is no way to get container namespace information
  for the Plugin at allocation time. This may change with future kubernetes
  versions.
+ `cexmode`: optional, specifies the required CEX mode if given and needs to be
  one of `ep11`, `cca` or `accel`. Enables an additional check step every time
  the APQNs on each node are screened by the CEX device plugin: all APQNs
  of this set are checked to match the CEX mode. On mismatch the CEX plugin
  informs via log entry and discards the use of this APQN for the set.  
  TODO for v1.0 !!!
+ `mincexgen`: optional, specifies the minimal CEX card generation for this
  set. If given needs to match `cex[4-7]`. Enables an additional check step
  every time the APQNs on each node are screened: all APQNs of this set are
  checked to have at least the given generation of cex type. On mismatch the
  CEX plugin informs via log entry and discards the use of the APQN for this
  set.  
  TODO for v1.0 !!!
+ `apqns`: A list of APQN entries is listed here. The list may be empty (and
  exist only for future extensions) but should usually include at least one APQN
  entry. The list given here is assumet to be a list of equivalent APQNs which
  simple means every container requesting a CEX resource from this set can be
  made happy with anyone of these APQNs. What exactly equvialent means depends
  on the application and usage pattern and types of CEX resources. For CCA APQNs
  this usually means to have same CCA AES (and maybe ECC and other key types as
  well) Master Key setup together with same ACP (Access Control Point)
  settings. Similar EP11 APQNs may require to have the very same Master Wrapping
  Key and Control Point settings. Accelerator mode APQNs don't have any states
  or tweaks and can be freely exchanged by each other with no security
  exposure.

  The easiest APQN entry may comprise these two fields:
  + `adapter`: required, the crypto card number, often referred to as `adapter`
     number, a simple number in the range of 0-255.
  + `domain`: required, the domain within the adapter to use, a simple number in
    the range of 0-255.

  This pair of numbers is uniquely identifying an APQN within one s390 machine
  instance. If the cluster's worker nodes are distributed over more than one
  s390 machine, an additional field may be needed to distinguish an APQN(a,d)
  from maschine 1 from APQN(a,d) on maschine 2:
  + `machineid`: optional, is only required when the worker nodes are physically
  located on different z machines and the APQN pairs (adapter, domain) are no
  longer unique. If given, needs to match to  
  `<manufacturer>-<machinetype>-<sequencecode>`  
  with  
  `<manufacturer>` – value of the `Manufacturer` line from `/proc/sysinfo`  
  `<machinetype>` – value of the `Type` line from `/proc/sysinfo`  
  `<sequencecode>` – value of the `Sequence Code` line from `/proc/sysinfo`  
  For example a valid machineid string may be `IBM-3906-00000000000829E7`.

  Instead of the `adapter` field a `serialnr` field can be used:
  + `serialnr`: can be used instead of an `adapter` field, specifies the serial
    number of the crypto card as listed in the respective sysfs file
  `/sys/devices/ap/cardxx/serialnr`. For example `93AABEET` is a valid serial
  number string. As the serial number of a crypto card is worldwide unique there
  is no need to specify a `machineid` value, however a `domain` value is
  required to identify an APQN with the pair (serialnr,domain).  
  TODO for v1.0 !!!
+ `ccaaesmkvp`: may be given **INSTEAD** of a list of APQNs, specifies the CCA
  AES Master Key Verification Pattern against which all APQNs on all the worker
  nodes are matched. This is the hex value as it is listed for each CCA queue
  `xx.yyyy` in the respective sysfs file `/sys/devices/ap/cardxx/xx.yyyy/mkvps` in
  the `AES CUR` line. For example `0xb072bc5c245aac8a` could be a valid
  setting.  
  TODO for v1.0 !!!
+ `ep11wkvp`: may be given **INSTEAD** of a list of APQNs, specifies the EP11
  Master Wrapping Key Verification Pattern against which all APQNs on all the
  worker nodes are matched. This is the hex value as it is listed for each EP11
  queue `xx.yyyy` in the respective sysfs file
  `/sys/devices/ap/cardxx/xx.yyyy/mkvps` in the `WK CUR` line. For example
  `0xef490ddfce10b330b86cfe6db2ae2db98d65e8c19d9cb7a1b378dec93e398eb0` could be
  a valid setting.  
  TODO for v1.0 !!!

Ideally the list of APQNS within each set should be widely spread over all the
worker nodes of the cluster to have a maximal backup capability in case of
worker node overload or disturbance.

As the CEX plugin is hotplug aware, it is no problem to list APQNs which are
(currently) not available but will become alive in the future. More details
about hotplug and healty state of APQNs can be found in the details chapter.

A sample CEX resource configuration map can be found in the appendix.

The clusterwide CEX resource config map is provided to the CEX device plugin
daemonset as a volume which shadows the crypto config sets into a config file
`/work/cex_resources.json`. This config file is read only once every CEX device
plugin daemonset instance start. So changes on the clusterwide CEX resource
config map are not pushed and evaluated by allready running plugin
instances. More details and solutions can be found in the details chapter.

### CEX resource allocation by crypto load containers

Each container within a pod definition of a customer may request **one** APQN
within **one** defined crypto config sets. This is done with a `resources`
statement as part of the container's image declaration:

    spec:
      containers:
      - image: 'customer-container-image'
        imagePullPolicy: Always
        name: whatever
        ...
        resources:
          limits:
            ibm.com/s390/cex-config/<name_of_the_cex_crypto_config_set>: 1
        ...

The CEX plugin daemonset instances together with the kubernetes infrastructure
will:

1. Check the currently available and healthy APQNs within the given CEX config
   set and randomly choose one free APQN from the set. If all APQNs from the set
   are occupied the container start will be delayed until a resource becomes
   available.
2. The instance of the CEX daemonset on the node where this APQN is located will
   claim the APQN and run some preparation steps (create a new CEX device device
   node, prepare sysfs shadow dirs, bookkeeping).
3. The container will get started on the node where this CEX resource is
   physically located. It will get a device `/dev/z90crypt` with access to
   exactly this one APQN on the node and it will 'see' only this one APQN in
   it's sysfs subtrees `/sys/devices/ap` and `/sys/bus/ap`.

During the runtime of the container it will get monitored by the CEX device
plugin instance of the worker node. There is no active surveillance, the
monitoring is done for bookkeeping.

After termination of the container, the CEX device plugin instance on the worker
node will cleanup the resource allocations on the worker node (destroy the CEX
device node, remove sysfs shadow dirs, bookkeeping).

Please note that only one CEX resource can be allocated per container. Crypto
loads which would require to use CEX resources from different CEX config sets
need to be split into individual containers. As a pod definition may comprise
multiple containers and the containers can freely talk to each other this should
not be a hard limitation at all.

A sample crypto container load which requests a CEX resource can can be found in
the appendix.

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
                "project":   "blue_customer",
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
                "project":   "red_customer",
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
                "project":    "green_customer",
                "cextype":    "cca",
                "ccaaesmkvp": "0xb072bc5c245aac8a"
            },
            {
                "setname":   "yellow",
                "project":   "yellow_customer",
                "ep11wkvp":
            }
        ]
    }

#### Sample crypto load container

    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: testload-for-blue-customer
      namespace: default
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: testload-for-blue-customer
      strategy:
        type: Recreate
      template:
        metadata:
          labels:
            app: testload-for-blue-customer
        spec:
          containers:
          - image: 'bash'
            imagePullPolicy: Always
            name: testload-for-blue-customer
            command: ["/bin/sh", "-c", "while true; do echo test-crypto-load; sleep 30; done"]
            resources:
              limits:
                ibm.com/s390/cex-config/blue: 1

#### Sample plugin deployment


####
control domain
