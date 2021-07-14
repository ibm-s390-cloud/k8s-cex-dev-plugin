# Recognized Environment Variables

The plugin exposes several configuration options via environment
variables that can be adjusted during deployment.  Following
environment variables are recognized.  All environment variables have
default values that will be used if the deployment does not specify
these variables.  Additionally, some safety checks are in place to
either ensure the value is sanitized or prevent the plugin from
starting.

## APQN_OVERCOMMIT_LIMIT

How many virtual devices to create for one APQN.  If this value is 1,
no over-commit is used.  Values greater than one lead to over-commit
of the APQN.  Values less than 1 will be changed to 1.

Default: 1
Unit: none
Range: >= 1
Range Violation: Adjust to 1

## APQN_CHECK_INTERVAL

Time interval in seconds between health checks of the devices managed
by the plugin.

Default: 30
Unit: seconds
Range: >= 10
Range Violation: Adjust to 10

## RESOURCE_DELETE_NEVER_USED

Time interval in seconds after which resources that were created due
to a reservation but never used since, e.g., the requesting container
never started are destroyed.  Keep this limit to a reasonable time
since there is a natural delay between creation of the resource and
its usage inside the starting container.

Default: 1800
Unit: Seconds
Range: >= 30
Range Violation: Adjust to 30

## RESOURCE_DELETE_UNUSED

Time interval in seconds after which resources that were used by a now
terminated container get destroyed.

Default: 120
Unit: Seconds
Range: >= 30
Range Violation: Adjust to 30

## PODLISTER_POLL_INTERVAL

Time interval in seconds how often the plugin will poll existing pods
and used resources.  This has a direct connection to resource
reclamation since knowledge of existing pods and the resources used by
these pods is a prerequisite of the detection of unused resources.  As
a general guideline, keep this interval smaller than the
RESOURCE_DELETE_UNUSED timeout.

Default: 30
Unit: Seconds
Range: >= 10
Range Violation: Adjust to 10

## SHADOWSYSFS_BASEDIR

Basic directory for the shadow sysfs for this container.  The
directory must exist and be readable, writable, and executable.
Otherwise, the plugin will refuse to start.

Default: /var/tmp/shadowsysfs
Unit: none
Range: valid directory names
Range Violation: Abort plugin start
