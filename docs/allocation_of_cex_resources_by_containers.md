# Allocation of CEX resources by containers

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

For example, a container requesting a CEX resource from the config set
`CCA_for_customer_1` from the sample ConfigMap in appendix
[Sample CEX resource Configuration Map](appendix.md#sample-cex-resource-configuration-map)
needs the following container specification:

    ...
    spec:
      containers:
      - image ...
        ...
        resources:
          limits:
            cex.s390.ibm.com/CCA_for_customer_1: 1
        ...

[Sample CEX crypto load container](appendix.md#sample-cex-crypto-load-container)
in the appendix is a simple but complete sample yaml file for a customer load
with CEX resource allocation.

When the Kubernetes system tries to run an instance of this container it
recognizes the resource limitation. The CEX device plug-in instances should have
registered plug-in-devices for each of the config sets, among them
plug-in-devices for the `CCA_for_customer_1`. The Kubernetes system does the
bookkeeping for all these devices and therefore knows, which devices are free
and which devices were announced by the CEX device plug-in instances. The
Kubernetes system chooses one compute node where a CEX device plug-in runs that
had announced one of the free plug-in devices and forwards an allocation request
to this plug-in.

The plug-in instance running on the compute node where the container gets
applied, prepares the CEX resource and the sysfs shadow directories for the
container, returns these to the Kubernetes system, and then the container is
started. The container will have a device node `/dev/z90crypt` customized to
have access to the allocated APQN and a customized `/sys/devices/ap` and
`/sys/bus/ap` providing a limited view of the AP/zcrypt world.

When the container finally finishes, the CEX device plug-in on the compute node spots
this, cleans up the allocated resources, and the Kubernetes system marks the
plug-in-device as unused. The allocated resources which are cleaned up are the
customized additional zcrypt device node and the sysfs shadow dirs.

## Frequently asked questions

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
config set case. The pod is in `pending` state until a config set with
this name and a free CEX resource for this set come into existence. Then the
CEX resource is assigned and the container started.

Q: I'd like to assign more than one APQN to the container to provide a backup
possibility for the running application. Is this supported?

A: Currently, exactly **one** CEX resource can be requested by **one** container. The
idea for backups for cluster applications is to schedule more
pods/containers. This keeps the application within a container simple and easy
and delegates the backup and performance issues to the cluster system.

Q: I'd like to package an application into a container that uses different kinds
of CEX resources, for example one CCA and one EP11 APQN. So I'd like to assign
two APQNs from different config sets to one container. Does that work?

A: No. Currently, exactly **one** CEX resource can be assigned to **one**
 container. This is only a limit to containers, but not to pods. As a pod can
 contain several containers each container can request one CEX resource from any
 config set. Split your application into units using only one type of CEX
 resource and package each unit into it's own container. Now your pod load
 runs as multiple containers with each having it's own CEX resource.
