# Setting up the environment for the CEX device plug-in {: #setting-up-the-environment-for-the-cex-device-plug-in}

## CEX resources on IBM Z and LinuxONE {: #cex-resources-on-ibm-z-and-linuxone}

IBM Z and LinuxONE machines can have multiple Crypto Express cards (CEX) plugged in. The CEX device plug-in supports Crypto Express generations *CEX4* to *CEX7*. For each card, a mode of operation must be chosen: *Accelerator* mode, *CCA mode*, or *EP11* mode.

Each card is logically partitioned into independent units, so called crypto domains, which represent independent Hardware Security Modules (HSMs). These independent units within a card share the same mode of operation and the same card generation.

Thus, one HSM unit can be addressed with the *adapter* number (the crypto card
number within a machine) and the *domain* number (the crypto partition
index). Both values must be numeric in the range 0-255. They act as a link to one
HSM unit and are called an "*APQN*".
<!-- last sentence needed it confuses me and APQN was mentioned earlier. -->

An LPAR within an IBM Z or LinuxONE machine can have one or more crypto cards assigned and
one or more domains. This results in a 2-dimensional table of APQNs.

The Kubernetes cluster is implemented as a KVM host running on an LPAR. The control plane
and compute nodes of the cluster are represented by KVM guests running at and
controlled by the KVM host. Therefore, some or all of the crypto resources available
on the LPAR must be provided for use by the KVM guests running as Kubernetes
compute nodes.
<!-- point of view? -->
The point of view for a KVM guest running as a Kubernetes compute node is similar to the view of the LPAR. A compute node might have zero or more crypto adapters assigned and zero or more domains, which can be seen as a 2-dimensional table of APQNs.

This documentation does not cover the assignment and distribution of crypto resources to LPARs, KVM hosts, and KVM guests. For details, see: 

* Section 10.1.3 "Configuring Crypto Express7S" in the IBM Redbook [IBM z15 Configuration Setup](https://www.redbooks.ibm.com/abstracts/sg248860.html)

* [Configuring Crypto Express Adapters for KVM Guests](https://www.ibm.com/docs/en/linux-on-systems?topic=kvm-configuring-crypto-express-adapters-guests)

For more information on Crypto Express cards, generations, and operation modes see: 
* https://www.ibm.com/security/cryptocards

Usually the adapter/domain pair is sufficient to identify an APQN. However, if the compute nodes of a cluster are distributed over multiple IBM Z or LinuxONE machines a unique machine identification (*machine-id*) is needed in addition to the adapter and domain information.

An HSM contains a *secret* which must not get exposed to anyone. The secret, and potential additional settings of the HSM, are maintained by the *Security Administrator* of the system. These settings are typically done out-of-band, are properly maintained, and relatively static. On IBM Z and LinuxONE everything regarding crypto cards is typically done by the Security Administrator with the help of a Trusted Key Entry (TKE) workstation. For details, see the IBM Redbook [System Z Crypto and TKE Update](https://www.redbooks.ibm.com/abstracts/sg247848.html).

The *secret* is usually the source of a secret key often referred to as the *master key* or *master wrapping key*. Applications working with the HSM use *secure key* objects, which are clear key values encrypted ("wrapped") by the master key. Such a secure key can only be used together with the HSM as only the HSM has the master key to unwrap the secure key blob during a cryptographic operation.

A CEX crypto card in EP11 mode contains one *wrapping key*. A crypto domain on a CCA coprocessor card contains up to four master keys, which can be of type DES, AES, RSA, and ECC. Each of these master keys can wrap any type of clear key into a secure key.  A CEX card in accelerator mode does not contain any secrets and can only be used to accelerate RSA clear key operations.

Multiple HSMs can be set up by the security administrator to be used as a backup for each other. Thus, the master keys and additional settings can be *equal*. *Equal* in this context means that an application using secure key methods can fulfill the job with either one of these HSMs, which form an equivalence set. Spreading these equal APQNs among the compute nodes allows the Kubernetes dispatching algorithm to choose the target node of a crypto load. The algorithm is based on criteria like CPU and memory requirements and the availability of crypto resources.

In version 1 of the CEX device plug-in, a container should not change the configuration, master key, and control points of the HSM resources. Also any changes to crypto resources (mode, master keys, control points) should be performed while the affected APQNs are not available for use within the cluster.

<!-- xxx z/VM is not described here in any way -->
<!-- xxx it would also be nice to have an example here with some drawings -->
