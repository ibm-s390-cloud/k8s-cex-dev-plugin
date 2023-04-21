# Getting started with the CEX device plug-in

## Creating and establishing a CEX resource configuration map

The CEX device plug-in needs to know a valid CEX configuration to start up
properly. This section deals with creating a CEX resource configuration.

In the CEX resource configuration, equivalent APQNs are grouped into equivalence
sets, called *crypto config sets*. A *crypto config set* has a unique name and
comprises of one or more equivalent APQNs. A pod with a crypto application
requests a CEX resource by requesting the allocation of one arbitrary APQN from
the *crypto config set* by the name of the *crypto config set*.
<!-- ?? from/by ?? -->

### Considerations for equally configured APQNs

Within each config set, all the APQNs must be set up consistently.
For each CEX mode, consider:
- For Common Cryptographic Architecture (CCA) CEX resources, the
  master keys and access control point settings should be equal.
- For EP11 CEX resources, the EP11 wrapping key and control settings should be equal.
- CEX accelerator resources are stateless and do not need any equal setup.

A container requests exactly **one** *crypto config set* and obtains **one** CEX
crypto resource from the CEX device plug-in if an APQN is available, healthy,
and not already allocated. The APQN is randomly chosen and is assigned to the
container.

The cluster-wide configuration of the CEX crypto resources is kept in a
Kubernetes `ConfigMap` within the same namespace as the device plug-in.
If the Kustomize base deployment provided in
[git repository](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin) is used,
the namespace is called `cex-device-plugin`. The name of the `ConfigMap` must be
`cex-resources-config` and the content is a configuration file section in JSON
format.

A working sample is provided in the appendix
[Sample CEX resource configuration map](appendix.md#sample-cex-resource-configuration-map).

The following example shows only the head and some possibly *crypto config set*
definitions:

~~~json

    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: cex-resources-config
      namespace: cex-device-plugin
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
~~~

The `ConfigMap` defines a list of configuration sets. Each configuration
set comprises the following entries:

### Basic parameters

- `setname`: required, can be any string value, must be unique within
  all the configuration sets. This is the identifier used by the
  container to request one of the CEX crypto resources from within the
  set.
- `project`: required, can be any string value, namespace of the
  configuration set. Only containers with matching namespace can
  access CEX crypto resources of the configuration set. For version 1
  this is not fully implemented as there are limits on the existing
  API preventing this. For details, see: [Limitations](technical_concepts_limitations.md#limitations).
- `cexmode`: optional, specifies the CEX mode. If specified, one of the
  following choices is required: `ep11`, `cca`, or `accel`.
  Adds an extra verification step every time the APQNs on each node are screened
  by the CEX device plug-in. All APQNs of the configuration set must match the
  specified CEX mode. On mismatches, the CEX device plug-in creates a log entry
  and discards the use of this APQN for the configuration set.
- `mincexgen`: optional, specifies the minimum CEX card generation for the
  configuation set. If specified, must match to `cex[4-9]`.
  Adds an extra verification step every time the APQNs on each compute node are
  screened. All APQNs of the configuration set are checked to have at least the
  specified CEX card generation. On mismatches, the CEX device plug-in creates a
  log entry and discards the use of the APQN for the configuration set.
- `overcommit`: optional, specifies the overcommit limit for resources in
  this ConfigSet. If the parameter is omitted, it defaults to the value
  specified through the environment variable APQN_OVERCOMMIT_LIMIT. If the
  environment variable is not specified, the default value for overcommit is 1
  (no overcommit).

### APQN parameters

* `apqns`: A list of equivalent APQN entries. The exact meaning of *equivalent*
  depends on the crypto workload to be run with the *crypto config
  set*. However, it forms a set of APQNs where anyone is sufficient to fulfill
  the needs of the requesting crypto workload container. See
  [Considerations for equally configured APQNs](#considerations-for-equally-configured-apqns).

  For example, a CCA application that uses a given AES secure key always relies
  on APQNs with a master key that wraps this secure key, regardless on which
  container it runs. In other words the master key setup of the APQNs within a
  ConfigSet should be the same.

  <!-- RB: to be discussed delete first sentence An APQN ... config set. -->
  An APQN must not be member of more than one *crypto config set*. It is valid
  to provide an empty list. It is also valid to provide APQNs, which might
  currently not exist but might come into existence sometime in future when new
  crypto cards are plugged.

  The most simple APQN entry comprises these two fields:
  - `adapter`: required, the CEX card number. Can be in the range of 0-255.
    Typically referred to as `adapter` number.
  - `domain`: required, the domain on the adapter. Can be in the range of 0-255.

  The tuple of these two numbers uniquely identifies an APQN within one hardware
  instance. If the compute nodes are distributed over more than one hardware
  instance, an extra entry is needed to distinguish an APQN(a,d) on hardware
  instance 1 from APQN(a,d) on hardware instance 2:

  - `machineid`: optional, is only required when the compute nodes are
    physically located on different hardware instances and the APQN pairs
    (adapter, domain) are not unique. If specified, the value must be entered as
    follows:
    `<manufacturer>-<machinetype>-<sequencecode>`
    with
    - `<manufacturer>` – value of the `Manufacturer` line from `/proc/sysinfo`
    - `<machinetype>` – value of the `Type` line from `/proc/sysinfo`
    - `<sequencecode>` – value of the `Sequence Code` line from `/proc/sysinfo`

    For example, a valid value for `machineid` is `IBM-3906-00000000000829E7`.

    The tuple (a,d) gets extended with the machine id, which is unique per
    hardware instance and the triple (a,d,maschineid) identifies an APQN again
    uniquely within the hardware instances.

<!-- BEGIN - Commented out. This is stuff for future development
  Instead of the `adapter` field a `serialnr` field can be specified:
  + `serialnr`: specifies the serial number of the crypto card as listed in
    the respective sysfs file `/sys/devices/ap/cardxx/serialnr`.
    For example, `93AABEET` is a valid serial number string. The serial number
    of a CEX crypto card is unique world-wide.
    A `domain` value is required to identify an APQN using (serialnr,domain).
    xxx an explanation of how to obtain the serial number for a card should
    be here also - TKE ? sysfs  -->

<!--### Alternative options to identify APQNs
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
  TODO for v1.1 !!! -->

<!-- RB: to be discussed
An APQN must not be member of more than one crypto config set.  In the absense of any other
parameters (like cexmode and mincexgen) that means an APQN identifier must not be a member
of more than one crypto config set. If an APQN identifier is a member of more than one crypto
config set, then each set must contain additional paramerters such that only one crypto
config set declares a valid APQN with said APQN identifier (e.g. each crypto config set
including APQN identifier (1,1) has a different cexmode).
Note, version 1 will enforce that no APQN identifier is a memeber of more than one crypto config set.
-->

### Establishing the CEX resource configuration map

The CEX resource configuration map is a Kubernetes `ConfigMap` named
`cex-resources-config` in the same Kubernetes namespace as the CEX device
plug-in.

The [CEX plug-in git repository](https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin)
contains a Kustomize base deployment that generates a configuration
map from a given `cex_resources.json` file.

1. Download the repository and go to the `deployments/configmap` folder.
2. Edit the `cex_resources.json` JSON file in the `deployments/configmap` folder
   with your favorite editor.
3. To verify via pretty-print that you made valid JSON entries without errors,
   run the following command: <br>`jq -r . cex_resources.json`<br>
   If you see error messages, you need to fix them before continuing to the next step.
4. To create the configurtion map, run the following command: <br>`oc create -k .`<br>
   To update an already existing config map, run the following command: <br>`oc apply -k .`<br>
