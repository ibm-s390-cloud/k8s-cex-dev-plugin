# Kubernetes device plug-in for IBM CryptoExpress (CEX) cards

This publication provides information on version 1 of the Kubernetes device plug-in for IBM
CryptoExpress (CEX) cards available for IBM Z and LinuxONE (s390x).

Authors:

- Harald Freudenberger <freude@linux.ibm.com>
- Juergen Christ <jchrist@linux.ibm.com>

Changelog:- [Kubernetes device plug-in for IBM CryptoExpress (CEX) cards](#kubernetes-device-plugin-for-ibm-cryptoexpress-cex-cards)
  - [Overview](#overview)
  - [Setting up the environment for the CEX device plug-in](#setting-up-the-environment-for-the-cex-device-plug-in)
    - [Overview on hardware, CEX crypto resources, and Kubernetes compute nodes](#overview-on-hardware-cex-crypto-resources-and-kubernetes-compute-nodes)
    - [Assignment of CEX crypto resources to LPARs and KVM](#assignment-of-cex-crypto-resources-to-lpars-and-kvm)
    - [Distributing CEX crypto resources for compute nodes across physical](#distributing-cex-crypto-resources-for-compute-nodes-across-physical)
    - [CEX crypto resources](#cex-crypto-resources)
  - [Installing the CEX device plug-in](#installing-the-cex-device-plug-in)
  - [Configuring the CEX device plug-in](#configuring-the-cex-device-plug-in)
    - [Requirements for equally configured APQNs](#requirements-for-equally-configured-apqns)
      - [Sample flow how an APQN is assigned to a container application](#sample-flow-how-an-apqn-is-assigned-to-a-container-application)
    - [Creating configuration sets of CEX crypto resources](#creating-configuration-sets-of-cex-crypto-resources)
      - [Basic parameters](#basic-parameters)
      - [APQN parameters](#apqn-parameters)
      - [Alternative options to identify APQNs](#alternative-options-to-identify-apqns)
    - [Further considerations](#further-considerations)
  - [Using the CEX device plug-in](#using-the-cex-device-plug-in)
    - [Requesting crypto configuration sets for container applications](#requesting-crypto-configuration-sets-for-container-applications)
  - [Details and technical reference](#details-and-technical-reference)
    - [config map details](#config-map-details)
      - [optional fields](#optional-fields)
      - [Updates and Rolling out changes on the clusterwide CEX resource configuration](#updates-and-rolling-out-changes-on-the-clusterwide-cex-resource-configuration)
    - [CEX device plug-in details](#cex-device-plug-in-details)
      - [config map](#config-map)
      - [CEX resource registration](#cex-resource-registration)
      - [APQN detection, hotplug and hotunplug, healthy state](#apqn-detection-hotplug-and-hotunplug-healthy-state)
      - [The CEX device plugin application](#the-cex-device-plugin-application)
      - [The shadow sysfs](#the-shadow-sysfs)
      - [podlister](#podlister)
      - [Ccustomization with environment variables](#ccustomization-with-environment-variables)
      - [Selinux and the Init Container](#selinux-and-the-init-container)
      - [All Volume Mounts required by the CEX Device Plugin](#all-volume-mounts-required-by-the-cex-device-plugin)
    - [Appendix](#appendix)
      - [Sample CEX resource configuration map](#sample-cex-resource-configuration-map)
      - [Sample crypto load container](#sample-crypto-load-container)
      - [Sample plugin deployment](#sample-plugin-deployment)
     - [Notices](#notices)

- 2021/07/08: first draft version

## Inclusive language

While IBM values the use of inclusive language, terms that are outside of IBM's direct influence are sometimes required for the sake of maintaining user understanding. As other industry leaders join IBM in embracing the use of inclusive language, IBM will continue to update the documentation to reflect those changes.

To learn more about this initiative, read the [Words matter blog on ibm.com](https://www.ibm.com/blogs/think/2020/08/words-matter-driving-thoughtful-change-toward-inclusive-language-in-technology/).

## Introduction

This Kubernetes device plug-in provides access to IBM s390 Crypto Express (CEX)
cards for s390 Kubernetes container loads. Throughout this publication, the term 'CEX device plug-in'
is used to refer to this Kubernetes device plug-in.

### Overview
A CEX config map defines one or more crypto configuration sets. A crypto
configuration set consists of resources of one or more IBM CryptoExpress
cards. A set is identified by its name and is instantiated as a Kubernetes
extended resource. Each crypto configuration set provides a pool of equivalent
CEX crypto resources. Containers can request one item by requesting such an
Kubernetes extended resource.

The CEX device plug-in instances running on all compute nodes handle allocation
requests, check if the existing crypto resources are available on the nodes, 
claim the resource, and ensure containers are scheduled on the right nodes with
the requested CEX crypto resources.

The CEX device plug-in handles:
- Bookkeeping of the CEX crypto resources
- Monitoring of all containers with allocated CEX crypto resources
- Cleaning up after container runtimes ended
- Hot pluging (and unplugging) of CEX crypto resources

## Setting up the environment for the CEX device plug-in

Before you set up the CEX device plug-in, the CEX crypto resources on the
compute nodes of the cluster need to be set up. See xxx for details. 

<!--This is not specific to the use of the CEX device plug-in and, thus, below a brief summary with
references to existing documentation-->

### Overview on hardware, CEX crypto resources, and Kubernetes compute nodes

Make sure to read and understand these additional resources before you install the CEX plug-in:  
- [IBM Systems cryptographic HSMs](https://www.ibm.com/security/cryptocards) 
- [Common Cryptographic Architecture functional overview](https://www.ibm.com/docs/en/linux-on-systems?topic=cca-overview)
- [Establing the security environment](https://www.ibm.com/docs/en/linux-on-systems?topic=key-establishing-security-environment)
- [Setting up the infrastructure](https://www.ibm.com/docs/en/linux-on-systems?topic=2020-setting-up-infrastructure)

xxx CEX crypto cards, modes of operation, domain partitioning, HSM and APQNs
CEX card Serialnumber, machine id

### Assignment of CEX crypto resources to LPARs and KVM 

<!-- taking out all z/VM mentions ...and z/VM-->

#### Assigning adapters and domains to LPARs

After you set up the Crypto Express adapter in the Support Element, you must allow access to it from your LPAR. You achieve this by using the Hardware Management Console (HMC) or the Support Element (SE).
You can define a certain LPAR to use a domain (or multiple domains) as a usage domain and as a control domain, or as a control domain only. 

- For details, see section 10.1.3 "Logical partition cryptographic definition" in the IBM Redbook [IBM z15 Configuration Setup](https://www.redbooks.ibm.com/redbooks/pdfs/sg248860.pdf)

#### Assign crypto to Virtual Function I/O (VFIO) 

KVM hosts can use the Virtual Function I/O (VFIO) framework and the VFIO mediated device framework to pass host devices with their attributes through to their KVM guests.

- For details, see [KVM virtual server integration - Assigning cryptographic adapter resources to vfio_ap](https://www.ibm.com/docs/en/linux-on-systems?topic=setup-assign-crypto-vfio)

<!-- xxx z/VM guest crypto resources xxx (see docu xxx)-->

### Distributing CEX crypto resources for compute nodes across physical
and/or virtual machine boundaries

xxx Distributed cluster compute nodes across several physical and/or virtual machines
and APQN address clashing

xxx APQN address clashing

### CEX crypto resources

xxx What comes here???

## Installing the CEX device plug-in

xxx TODO Provide more details on how to install the plug-in... will be more
    easier once the container image is out

Document here:
- daemonset

## Configuring the CEX device plug-in

The configuration of CEX crypto resources that are handled by the CEX device plug-in 
is defined in a config map that uses the JSON format.  The configuration defines
one or more unique `cryptoconfigsets`. Each set groups CEX crypto
resources that consist of associated cryptographic coprocessors (APQNs).
<!---SN: wherever the first mention of APQN will be should be written out at firs occurance-->

### Requirements for equally configured APQNs

Within each configuration set, all the APQNs must be set up consistently. 
For each CEX mode, consider:
- For Common Cryptographic Architecture (CCA) CEX resources, the master keys and access control points
  should be equal.
- For EP11 CEX resources, EP11 wrapping key and control settings should be equal.
- CEX accelerator resources are stateless and do not need any equal setup.

A container must request exactly **one** configuration set and obtains **one**
CEX crypto resource by the CEX device plugin if one APQN is available,
healthy, and not already allocated. The APQN is randomly choosen and is assigned to the life time of the container.

#### Sample flow how an APQN is assigned to a container application

<!--SN: Do we have a flowchart we could add here?-->

The cluster administrator defines a configuration set named *blue* with a
set of APQNs and a set named *red*. This differentiation reflects
different tenants (*blue* and *red*) with dedicated CEX crypto resources for
each of them. The *blue* configuration set is used for applications (containers) from tenant *blue*, the *red* configuration set is
used for applications by tenant *red*.  Each APQN within the *blue* and *red*
configuration set is assumed to be equal. That means all *blue* applications 
can perform the correct crypto operations with any of the APQNs from the *blue*
configuration set. The same applies to application containers using APQNs from
the *red* configuration set.

At start of a container requesting the *blue* configuration set, the CEX
device plugin gets an allocation request for an *APQN* of the *blue*
configuration set.  The CEX device plugin chooses one random APQN, which is
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
config file section in JSON format. 

* JSON ConfigMap example:

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

The config map defines a list of configuration sets. Each configuration set
comprises these entries:
<!--Maybe have these as tables-->
#### Basic parameters

- `setname`: required, can be any string value, must be unique within all
  the configuration sets. This is the identifier used by the container to request
  one of the CEX crypto resources from within the set.

- `project`: required, namespace of the configuration set. Only containers with
  matching namespace can access CEX crypto resources of the configuration set.
  Currently this option is not implemented due to missing namespace information
  at allocation time. This might change with future Kubernetes versions.
  <!--How can this be required if it's not implemented yet?-->

- `cexmode`: optional, specifies the CEX mode. If specified, one of the following choices is required: `ep11`, `cca`, or `accel`. 
  Adds an extra verification step every time
  the APQNs on each node are screened by the CEX device plugin. All APQNs
  of the configuration set must match the specified CEX mode. On mismatches,
  the CEX device plugin creates a log entry and discards the use of this
  APQN for the configuration set.
  TODO for v1.1 !!!

- `mincexgen`: optional, specifies the minimal CEX card generation for the
  configuation set. If specified, must match to `cex[4-7]`. Adds an
  extra verification step every time the APQNs on each compute node are screened.
  All APQNs of the configuration set are checked to have at least the
  specified CEX card generation. On mismatches, the CEX device plugin creates
  a log entry and discards the use of the APQN for the configuration set.
  TODO for v1.1 !!!

#### APQN parameters

- `apqns`: A list of APQN entries. The list can be empty, and
  exist only for future use, but should typically include at least one
  APQN entry. The specified list is assumed to be a list of equivalent APQNs.
  See also [Considerations for equally configured
  APQNs](#considerations-for-equally-configured-apqns).

  The most simple APQN entry comprises these two fields:
  + `adapter`: required, the CEX card number. Can be in the range of 0-255.
    Typically referred to as `adapter` number.
  + `domain`: required, the domain on the adapter. Can be in the range of 0-255.

  The tuple of these two numbers uniquely identifies an APQN within one
  hardware instance. If the compute nodes are distributed over more than one
  hardware instance, an extra entry is needed to distinguish an
  APQN(a,d) on hardware instance 1 from APQN(a,d) on hardware instance 2:

  + `machineid`: optional, is only required when the compute nodes are physically
  located on different hardware instances and the APQN pairs (adapter, domain)
  are not unique. If specified, the value must entered as follows:
  
    `<manufacturer>-<machinetype>-<sequencecode>`
   with
    - `<manufacturer>` – value of the `Manufacturer` line from `/proc/sysinfo`
    - `<machinetype>` – value of the `Type` line from `/proc/sysinfo`
    - `<sequencecode>` – value of the `Sequence Code` line from `/proc/sysinfo`  
  
    For example, a valid value for `machineid` is `IBM-3906-00000000000829E7`.

  Instead of the `adapter` field a `serialnr` field can be specified:

  + `serialnr`: specifies the serial number of the crypto card as listed in
  the respective sysfs file `/sys/devices/ap/cardxx/serialnr`.  
  For example, `93AABEET` is a valid serial number string. The serial number
  of a CEX crypto card is unique world-wide.  
  A `domain` value is required to identify an APQN using (serialnr,domain).
  TODO for v1.1 !!!

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

### Further considerations

For high availability and scaling of container applications using crypto
configuration sets, distribute equally configured APQNs widely across all
the compute nodes in the Kubernetes cluster.

The CEX device plugn is hotplugging-aware. You can list APQNs that are not
yet available but might become alive later. For details, see Details
section below.
TODO: add reference to details section

For a sample config map with CEX, crypto configuration sets can be found
in the appendix.

The clusterwide CEX resource config map is provided to the CEX device plugin
daemonset as a volume, which shadows the crypto config sets into a config file
`/work/cex_resources.json`. This config file is read only once every CEX device
plugin daemonset instance start. So changes on the clusterwide CEX resource
config map are not pushed and evaluated by already running plugin
instances. More details and solutions can be found in the details section.

## Using the CEX device plug-in

### Requesting crypto configuration sets for container applications

Each container within a pod definition can request exactly **one** APQN
from **one** of the defined crypto configuration sets. To enable the request for the CEX device plug-in add the `cex.s390.ibm.com/` prefix to the `resources` statement in the image declaration of the containers section.

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

The Kubernetes infrastructure and the CEX device plug-in perform the following steps:

1. The currently available and healthy APQNs of the specified crypto
   configuration set are verified.
2. A free APQN is randomly choosen from the configuration set.  If all APQNs
   are occupied, starting the container is delayed until an APQN becomes
   available.
3. The CEX device plugin instance, located on the APQNs compute, claims the APQN and prepares it for use.
   Note: Preparation includes creating a logical CEX device node, prepare shadow
   sysfs directories, and perform bookkeeping the assignment.
4. The container is started on the APQNs compute node. The container namespace contains the 
   `/dev/z90crypt` device.  Operations on the device are restricted to the
   assigned APQN only on the compute node. Furthermore, in the sysfs for the
   container namespace, only the assigned APQN is visible in the sysfs
   subtrees `/sys/devices/ap` and `/sys/bus/ap`.

The CEX device plug-in performs bookkeeping by monitoring of APQNs during the
runtime of containers.
When the container stops, the CEX device plug-in instance on the compute node
cleans up the resources that are created during the preparation step. The logical CEX device node is destroyed,  shadow sysfs directories and bookkeeping data are removed.

Note: One crypto configuration set can be allocated per container. You must 
decompose applications requiring two or more crypto configuration sets into
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

### CEX device plug-in details

#### config map

The config map is automatically reloaded without the need to restart
the plugin.  Reloading is configured via the environment variable
`CRYPTOCONFIG_CHECK_INTERVAL` which should be set to the value in
seconds after which the plug-in should check for updates.  The minimal
value for this variable is 120 seconds.  Note that after an update of
a config map, the cluster needs some time (typically up to 2 minutes)
to propagate the changes to all nodes.

Another (potentially faster) way to update the config map for a device
plugin is to restart the rollout of the deployment via
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
                "cexmode":   "cca",
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
                "cexmode":   "ep11",
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
                "cexmode":    "cca",
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

#### Sample plug-in deployment


####
control domain

# Notices

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

## Trademarks

Helm™ and the Helm logo are trademarks of The Linux® Foundation, and use of them as a trademark is subject to The Linux Foundation's Trademark Usage Guidelines at [https://www.linuxfoundation.org/trademark-usage/](https://www.linuxfoundation.org/trademark-usage/).

IBM, the IBM logo, and ibm.com® are trademarks or registered trademarks of International Business Machines Corp., registered in many jurisdictions worldwide. Other product and service names might be trademarks of IBM or other companies. A current list of IBM trademarks is available on the Web at [Copyright and trademark information](https://www.ibm.com/legal/copytrade) at www.ibm.com/legal/copytrade.shtml.

Java™ and all Java-based trademarks and logos are trademarks or registered trademarks of Oracle and/or its affiliates.

The registered trademark Linux is used pursuant to a sublicense from the Linux Foundation, the exclusive licensee of Linus Torvalds, owner of the mark on a worldwide basis.

Kubernetes® and the Kubernetes logo are registered trademarks of The Linux Foundation, and use of them as a trademark is subject to The Linux Foundation's Trademark Usage Guidelines at [https://www.linuxfoundation.org/trademark-usage/](https://www.linuxfoundation.org/trademark-usage/).

Red Hat®, OpenShift®, and Ansible® are registered trademarks of Red Hat, Inc. or its subsidiaries in the United States and other countries.

UNIX® is a registered trademark of The Open Group in the United States and other countries.

## Terms and conditions for product documentation

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



