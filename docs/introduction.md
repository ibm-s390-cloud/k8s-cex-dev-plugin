# Introduction {: #introduction}

This Kubernetes device plug-in provides access to IBM s390 Crypto Express (CEX)
cards for s390 Kubernetes container loads. Throughout this publication, the term
'CEX device plug-in' is used to refer to this Kubernetes device plug-in.


## Overview {: #overview}

The Kubernetes CEX device plug-in provides IBM Crypto Express cards to be made
available on Kubernetes nodes for use by containers.

The CEX device plug-in version 1 groups the available CEX resources (*APQNs*)
into *CEX config sets*. Containers can request **one** resource from **one**
*CEX config set*. Thus, from a container perspective, the APQNs within one *CEX config set* should be equivalent, which means that each APQN can be used interchangeably for any crypto workload.

See [Considerations for equally configured APQNs](getting_started_with_the_cex_device_plug_in.md#considerations-for-equally-configured-apqns) for details.

The CEX config sets are described in a cluster-wide `ConfigMap`, which is maintained by the cluster administrator.

The CEX device plug-in instances running on all compute nodes:
* Check if the existing crypto resources are available on the nodes.
* Handle CEX resource allocation requests from the containers.
* Claim the resource.
* Ensure containers are scheduled on the correct compute node with the requested CEX crypto resources.

The CEX device plug-in instances running on each compute node:
* Screen all the available CEX resources on the compute node and provide this information to the Kubernetes infrastructure service.
* Allocate and deallocate a CEX resource on request of the Kubernetes infrastructure based on the requirement of a pod asking for CEX support.

The application container only has to specify that it needs a CEX resource from a specific *CEX config set* with a Kubernetes resource limit declaration. The cluster system and the CEX device plug-in handle the details, claim a CEX resource, and schedule the pod on the correct compute node.

The following sections provide more information about CEX resources on IBM Z and LinuxONE,
the CEX device plug-in details, the CEX crypto configuration as *CEX config sets* in a cluster,
and application container handling details.
