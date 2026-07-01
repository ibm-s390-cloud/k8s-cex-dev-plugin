# Support for control domains

The CEX device plug-in supports configuring control domains for CCA and EP11
crypto resources through the optional `control_domain` parameter of a crypto
configuration set.

For general information about managing control domains for Crypto Express adapters, see the IBM documentation on [managing Crypto Express adapters with the TKE workstation](https://www.ibm.com/docs/en/linux-on-systems?topic=support-managing-crypto-express-adapters-tke-workstation).

The `control_domain` parameter controls which control domains are made
available to applications running inside the container. The configured
control-domain setting is applied when the CEX resource is allocated.

If the `control_domain` parameter is omitted, the default value is
`usage-only`.

Example:

~~~json
{
    "setname": "CCA_for_customer_1",
    "project": "customer-1",
    "cexmode": "cca",
    "control_domain": "usage-and-control",
    "apqns":
    [
        {
            "adapter": 1,
            "domain": 6
        }
    ]
}
~~~

## Supported control_domain values

The following values are supported for the `control_domain` parameter:

- `usage-only`

  No control domains are assigned to the container.

- `usage-and-control`

  The usage domain of the allocated APQN is also assigned as a control domain.

- `all-control-domains`

  All control domains available on the hosting node or LPAR are assigned to the
  container.

The `control_domain` parameter is only supported for crypto configuration
sets with `cexmode` set to `cca` or `ep11`.

## Control-domain behavior

The CEX device plug-in configures the control-domain mask during resource
allocation.

The configured control-domain mask is applied consistently to both the zcrypt
device node and the shadow AP bus sysfs presented to the container.

The control-domain mask remains unchanged for the lifetime of the container.

When `all-control-domains` is specified, only control domains available on
the hosting node or LPAR are assigned. Control domains that are not available
on the hosting system cannot be accessed.

## Configuration recommendations

When using control domains, it is recommended that equivalent APQNs within the
same crypto configuration set reside on the same hosting node or LPAR.

If APQNs from multiple nodes or LPARs are grouped into the same configuration
set, the control domains available to a container may vary depending on which
APQN is allocated.
