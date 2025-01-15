# Technical Concepts and Limitations

## CEX configuration ConfigMap updates

From a cluster administration point of view it is desirable to change the CEX
configuration in the cluster-wide crypto ConfigMap. For example, to add or remove
CEX resources within a config set or even add or remove whole crypto config
sets.

This can be done during regular cluster uptime but with some carefulness. Every
`CRYPTOCONFIG_CHECK_INTERVAL` (default is 120s) the crypto ConfigMap is
re-read by all the CEX device plug-in instances. The new ConfigMap is verified and if
valid, activated as the new current ConfigMap. On successful ConfigMap
re-read the plug-in logs a message:
```
	CryptoConfig: updated configuration
```
If the verification of the new CEX ConfigMap fails, the CEX device plug-in logs an
error message. One reason for the verification failure might be the failure to
read or parse the ConfigMap resulting in error logs like:
```
	CryptoConfig: Can't open config file ...
```
or
```
	CryptoConfig: Error parsing config file ...
```
If the verification step fails, the following message is displayed:
```
	Config Watcher: failed to verify new configuration!
```
These failures result in running the plug-in instances without any configuration map.

The log messages appear periodically until yet another update of the ConfigMap
is finally accepted as valid.

**Note:** After an update of a configuration map, the cluster needs some time
(typically up to 2 minutes) to propagate the changes to all nodes.
Another, potentially faster, way to update the configuration map for the plug-in is to
restart the rollout of the deployment via:
```
	kubectl rollout restart daemonset <name-of-the-cex-plug-in-daemonset> -n cex-device-plugin
```
This triggers a restart of each instance of the daemonset in a coordinated way
by Kubernetes.


## Overcommitment of CEX resources

By default, a CEX resource (an APQN) maps to exactly one Kubernetes
*plug-in-device*. This is the administration unit known by Kubernetes and in
fact a container requests such a plug-in device.

By default, the CEX device plug-in maps each available APQN to one plug-in
device and as a result one APQN is assigned to a container requesting a CEX
resource.

The CEX device plug-in can provide more than one plug-in-device per APQN, which
allows some overcommitment of the available CEX resources.

Setting the environment variable `APQN_OVERCOMMIT_LIMIT` to a value greater than
1 (default is 1) allows to control how many plug-in devices are announced to the
Kubernetes system for each APQN. For example, with three APQNs available within a
config set and an overcommit value of 10, 30 CEX plug-in devices are
allocatable and up to 30 containers could successfully request a CEX resource.
The environment variable is specified in the DaemonSet YAML file via the `env`
parameter.

You can specify the optional ConfigSet parameter "overcommit" to control the
overcommit limit at config set level. If this parameter is omitted, the value
defaults to the environment variable.

Eventually, more than one container will share one APQN with overcommitment
enabled. This exposes no security weakness, but might result in lower
performance for the crypto operations within each container.

**Note:** Dynamically changing the overcommit value, either by changing the
environment variable, or by changing the overcommit parameter of a config set,
changes the number of available CEX resources. If the number of available resources
increases, containers waiting for resources might be able to run. Whereas
already running containers continue to run, even if a used resource is no more
available because of the decreased number of available resources. Due to lack
of resources, those containers cannot be restarted.

## The device node z90crypt

On a compute node, the device node `/dev/z90crypt` offers access to all zcrypt
devices known to the compute running as a KVM guest. The application of a
container, which requests a CEX resource will also see and use the device node
`/dev/z90crypt`. However, what is visible inside the container is in fact a newly
constructed z90crypt device with limited access to only the APQN assigned.

On the compute node, these constructed z90crypt devices are visible in the
`/dev` directory as device nodes
`zcrypt-apqn-<card>-<domain>-<overcommitnr>`. With the start of the container
the associated device node on the compute node is mapped to the `/dev/z90crypt`
device inside the container.

These constructed z90crypt devices are created on the fly with the CEX
allocation request triggered with the container start and deleted automatically
when the container terminates.

With version 1 of the CEX device plug-in, the constructed zcrypt device nodes limit
access to exact one APQN (adapter, usage domain, no control domain), allowing
all ioctls.

**Note:** These settings allow both usage and control actions, which are
restricted to the underlying APQN with the `/dev/z90crypt` device that is
visible inside the container, even with overcommited plug-in devices.


## The shadow sysfs

The CEX device plug-in manipulates the AP part of the sysfs that a container can
explore. The sysfs tree within a container contains two directories related to
the AP/zcrypt functionality: `/sys/bus/ap` and `/sys/devices/ap`.

Tools working with zcrypt devices, like `lszcrypt` or `ivp.e`, need to see the
restricted world, which is accessible via the `/dev/z90crypt` device node within
the container.

The CEX device plug-in creates a *shadow sysfs* directory tree for each of these
paths on the compute node at `/var/tmp/shadowsysfs/<plug-in-device>`. With
the start of the container, both directories `/sys/bus/ap` and
`/sys/devices/ap` are overlayed (overmounted) with the corresponding shadow
directory on the compute node.

These shadow directory trees are simple static files that are created from the
original sysfs entries on the compute node. They loose their sysfs functionality
and show a static view of a limited AP/zcrypt world. For example,
`/sys/bus/ap/ap_adapter_mask` is a 256 bit field listing all available adapters
(crypto cards). The manipulated file that appears inside the container only
shows the adapter that belongs to the assigned APQN. All load and counter values
in the corresponding sysfs attributes, for example
`/sys/devices/ap/card<xx>/<xx>.<yyyy>/request_count`, show up as 0 and don't get
updates when a crypto load is running.

This restricted sysfs within a container should be sufficient to satisfy the
discovery tasks of most applications (`lszcrypt`, `ivp.e`, opencryptoki with CCA
or EP11 token) but has limits. For example, `chzcrypt` will fail to change sysfs
attributes, offline switch of a queue will not work, and applications inspecting
counter values might get confused.

With *live sysfs support* enabled (which is now the default), the CEX device
plug-in instances use some mount feature to give the containers a live view
of some of the sysfs attribute files. See the following paragraph for more
details.

An administrator logged into a Kubernetes compute node could figure out the
assignment of a CEX resource and a requesting container. For example, by reading
the log messages from the plug-ins. Without overcommitment the counters of an
APQN on the compute node reflect the crypto load of the associated container and
`lszcrypt` can be used.

### Live sysfs support within the shadow sysfs

With *live sysfs support* enabled some of the files shadowed to the directories
`/sys/bus/ap` and `/sys/devices/ap` become alive. For example `online` attribute
reflects the real state of this APQN of the providing host node. In detail these
attributes are no longer static copies but show the *real* content:

* `sys/devices/cardxx/online`: Online state of the card (1 online, 0 offline)
* `sys/devices/cardxx/pendingq_count`: Number of pending requests queued into
  the hardware for this card.
* `sys/devices/cardxx/request_count`: Total number of requests processed for
  this card.
* `sys/devices/cardxx/requestq_count`: Number of pending requests queued into
  the software queue waiting for getting moved into the hardware queue for this
  card.
* `sys/devices/cardxx/xx.yyyy/`: This is the queue sub-directory offering a
  bunch of information about the queue device. For example the `mkvps` sysfs
  attribute file becomes alive and shows updates on the verification patterns
  when the master key(s) change via TKE for this APQN.

Live sysfs support can get enabled or disabled on a cluster scope with setting
the environment variable `APQN_LIVE_SYSFS` to 1 (enabled) or 0
(disabled). Please note that if this environment variable is not provided, the
CEX device plugin assumes to enable *live sysfs support*.

In addition *live sysfs support* can be tweaked per config set thus overwriting
the cluster *live sysfs support* default. There is a new field `livesysfs`
supported within a config set definition of the CEX resource definition
file. The following yaml fragment shows an example:

    ...
    "cryptoconfigsets":
    [
        {
            "setname":   "CEX_config_set_1",
            "project":   "customer-1",
            "cexmode":   "cca",
            "overcommit": 3,
            "livesysfs":  0,
            "apqns":
            [
                ...
            ]
        },
        {
            "setname":   "Another_CEX_config_set",
            "project":   "customer-2",
            "cexmode":   "ep11",
            "overcommit": 3,
            "livesysfs":  1,
            "apqns":
            [
                ...
            ]
        ...

If the `livesysf`field is not given within a config set definition, the default
value from the environment variable `APQN_LIVE_SYSFS` is used for the config
set.

Please note that the combination of *live sysfs support* and Overcommitment of
CEX resources may lead to the situation that sysfs counters are visible and can
be altered by multiple containers. For most applications this is not a problem
but especially performance measurements relying on correct counter values may
get confused.

## Hot plug and hot unplug of APQNs

The CEX device plug-in monitors the APQNs available on the compute node by
default every 30 seconds. This comprises the existence of APQNs and their
*online* state. When the compute node runs as a KVM guest it is possible to
*live* modify the devices section of the guest's xml definition at the KVM host,
which results in APQNs appearing or disappearing. The AP bus and zcrypt device
driver inside the Linux system recognizes this as hot plug or unplug of crypto
cards and/or domains.

It is also possible to directly change the *online* state of a card or APQN
within a compute node. For example, an APQN might be available but switched to
*offline* by intention by an system administrator.

A dialog on the HMC offers the possibility to *configure off* and *configure on*
CEX cards assigned to an LPAR. A CEX card in *config off* state is still visible
in the LPAR and thus in the compute node but similar to the *offline* state no
longer usable.

All this might cause the CEX device plug-in to deal with varying CEX
resources. The plug-in code is capable of handling hot plug, hot unplug, the
*online* state changes of CEX resources, and reports changes in the config set
to the Kubernetes system. Because of this handling, APQNs can be included into
the CEX config sets, which might not exist at the time of first deployment of
the CEX configuration map. At a later time the card is hot plugged and assigned
to the running LPAR. The cluster will spot this and make the appearing APQNs,
which are already a member in a config set, available for allocation requests.

The handling of the *online* state is done by reporting the relevant plug-in
devices as *healthy* (online) or *unhealthy* (offline). An *unhealthy*
plug-in device is not considered when a CEX resource allocation takes place.

**Note:** It might happen that a CEX resource becomes unusable (hot unplug or
offline state) but is assigned to a running container. The plug-in recognizes the
state change, updates the bookkeeping, and reports this to the
Kubernetes system but does **not** stop or kill the running container. It is
assumed that the container load fails anyway because the AP bus or zcrypt device
driver on the compute node reacts with failures on the attempt to use such a
CEX resource device. A well designed cluster application terminates with a
bad return code causing Kubernetes to re-establish a new container, which will
claim a CEX resource and the situation recovers automatically.


## SELinux and the Init Container

The CEX device plug-in prepares various files and directories that become mounted
to the pod at an allocation request. Among those mounts are the directories
descibed under [The shadow sysfs](#the-shadow-sysfs). These folders are
generated on the compute node and mounted into the new pod. In some cases,
special actions are needed for such a mount to be accessible inside the newly
created pod. For example, SELinux where the folder, or one of its parent
folders, must have the appropriate SELinux label. Other security mechanisms
might have different requirements.

Because the security mechanisms and their configuration depend on the
cluster instance, the CEX device plug-in does not provide any support
for such mechanisms.  Instead, in the SELinux case, an Init Container
can be used to set the correct label on the shadow sysfs root folder
`/var/tmp/shadowsysfs` that contains all the sub-folders that are
mapped into pods.  See [Sample CEX device plug-in daemonset
yaml](appendix.md#sample-cex-device-plug-in-daemonset-yaml) for an example of a
daemonset deployment of the CEX device plug-in that contains an init container
to set up `/var/tmp/shadowsysfs` for use in a SELinux-enabled environment.

## Limitations

### Namespaces and the project field

The `project` field of a CEX config set should match the namespace of the
container requesting a member of this set. This results in only *blue*
applications being able to allocate *blue* APQNs from the *blue* config set.

Unfortunately, the allocation request forwarded from the Kubernetes system to
the CEX device plug-in does not provide any namespace information. Therefore,
the plug-in is not able to check the namespace affiliation.

When the container runs, the surveillance loop of the CEX device plug-in detects
this mismatch and displays a log entry:
```
PodLister: Container <aaa> in namespace <bbb> uses a CEX resource <ccc> marked for project <ddd>!.
```

This behavior can be a security risk as this opens the possibility to use the
HSM of another group of applications. However, to really exploit this, more is
needed. For example, a secure key from the target to attack or the possibility to
insert a self made secure key into the target application.

As a workaround, you can set quotas for all namespaces except for the one
that is allowed to use the resource. See the following example:
```
	apiVersion: v1
	kind: ResourceQuota
	metadata:
	  name: cex-blue-quota-no-red
	  namespace: blue
	spec:
	  hard:
		requests.cex.s390.ibm.com/red: 0
		limits.cex.s390.ibm.com/red: 0
```
This yaml snippet restricts the namespace *blue* to allocate zero CEX resources
from the crypto config set *cex.s390.ibm.com/red*. The result is that all
containers, which belong to the *blue* namespace, are not able to allocate *red*
CEX resources any more.

[Sample CEX quota restriction
script](appendix.md#sample-cex-quota-restriction-script) in the appendix shows a
bash script that produces a yaml file, which establishes these quota
restrictions.
