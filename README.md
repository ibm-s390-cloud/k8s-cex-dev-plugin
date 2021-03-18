# Kubernetes Device Plugin for IBM CryptoExpress (CEX) cards (s390)


The Kubernetes CEX device plugin provides IBM CryptoExpress cards to be made
available on Kubernetes node(s) for use by containers.

*Please note that this is currently an early stage and subject to change.
Do not use in production environments.*

## How does the early stage work?

Build the plugin golang code and package the binary into a container. Deploy one
container per Kubernetes node for example as a daemon set in namespace
`kube-system`. By default, the plugin takes care of all APQNs of crypto cards in
`ep11` mode. With the command line option `-t <cextype>` or environment variable
`CEXTYPE`, you can switch the plugin to handle CCA-Coprocessor mode (`cca`) or
Accelerator mode (`accel`) cards respectively.

The plugin registers at the kubelet instance on the node as a device plugin
for the resource `ibm.com/cex-ep11` or `ibm.com/cex-cca` or
`ibm.com/cex-accel` dependent on the chosen cex type. Then, all available
crypto cards on the node are screened and a list of eligible APQNs is
generated and reported as 'plugin devices' to the kubelet. This screening runs
every 30 seconds and is capable to handle hot plug/unplug of crypto resources
on the node.

A container deployment can request one of these 'plugin devices' from the
kubelet by specifying resources limits like this:

```
  ...
  spec:
    containers:
    - image ...
      ...
      resources:
	limits:
	  ibm.com/cex-ep11: 1
      ...
```

The Kubernetes scheduler respects this resource limit and interacts with the
kubelets on the nodes to resolve this limitation. With the scheduler's
decision the kubelet on the node interacts with the plugin. The plugin
eventually creates a new zcrypt node device with the access restriction to the
APQN backing this 'plugin device' and links the container's `/dev/z90crypt` to
his new zcrypt node device. Effectively, the container is restricted to only use
this APQN for related crypto operations.

## Remarks and Limitations

* It's possible to deploy one plugin container responsible for EP11, another for
  CCA and a third one for Accelerators. A container using crypto
  resources can only request one of these, either an `ibm.com/cex-ep11` or an
  `ibm.com/cex-cca` or an `ibm.com/cex-accel` resource.
* This plugin stage does not care about master key settings on CCA or EP11
  crypto resources. This works fine with applications which do not require to
  use a crypto resource with defined master key settings. There is currently no
  way to influence the assignment of an APQN to an container requesting a crypto
  resource other than the cextype (ep11, cca or accel). For applications keeping
  secure keys which require a dedicated master key setup this approach only
  works when all possible APQNs share the very same master key settings (on all
  possible nodes). Ephemeral secure key creation and use of an container
  with crypto load isn't a problem as it is guaranteed that the backing crypto
  resource will not change during the lifetime of an container.
* A backing crypto resource might become invalid in some way. For example, the
  administrator switches the APQN or the crypto card 'offline' or the adapter is
  hot unplugged. The plugin detects this and informs the kubelet about the new
  list of available resources (leaving out the not anymore usable
  resources). Kubernetes does not actively kill the container using
  such a inactive resource. Instead Kubernetes assumes the container might
  struggle and fail using the resource and abort itself - and the container load
  should obey to this usage pattern.
* Currently, the plugin code has a hard coded limit of 1 for over-committing
  resources. This means that each backing APQN can only get assigned to one
  container requesting a cex resource. So when a Kubernetes node only has
  for example one APQN (of the cex type the plugin is handling) available the
  number of concurrently running containers requesting such a resource is
  limited to one instance on this node. A later version of this plugin
  might expose a possibility to over-commit APQNs to have more than one container
  using the same APQN.
* The crypto cards are accessed via device node `/dev/z90crypt`. At container
  start this device node is mapped to a new zcrypt node `/dev/zcrypt_apqn_x_y`
  which restricts the use to only use APQN(x,y). This works fine for all
  applications running ioctls only to the z90crypt device node. There exist
  applications which also screen the sysfs to find out the set of available
  cards and domains (for example `lszcrypt` uses the sysfs). The `/sys`
  directory as it appears inside containers is a simple read-only mapping from
  `/sys` on the node without any modifications. So `lszcrypt` shows all
  cards/domains which are available on the node but attempts to use them
  via ioctl() at the `/dev/z90crypt` device node within the container will fail
  for all but the one APQN allowed.
* The plugin is not informed by Kubernetes about the termination of containers
  using limited resources. So there is no way to have some bookkeeping about
  which 'plugin devices' and thus backing APQNs are unused and which ones are
  used. So there is also no way to remove no longer used zcrypt node devices. As
  it is allowed to shutdown and restart the plugin at any time, there is also no
  way to cleanup the zcrypt node devices with startup or termination.
  This is not a real issue, but users might wonder about the leftover
  `zcrypt_apqn_<adapter>_<domain>` device nodes in the nodes `/dev` directories.

## Authors

- Harald Freudenberger <freude@linux.ibm.com>
