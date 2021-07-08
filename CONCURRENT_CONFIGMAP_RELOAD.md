# Concurrent Reload of ConfigMap

Configuration of the cex device plugin is stored in a file called
cex_resources.json inside a config map.  The config map might be
updated to add or remove some APQNs or whole configuration sets.  Such
an update is then dynamically reloaded by the plugin after some time.
The time depends on the cluster configuration for the update of the
config map inside the pod and the setting of the environment variable
`CRYPTOCONFIG_CHECK_INTERVAL`.

# Configuration

The config map has to be mounted as a whole volume and not a `subPath`
mount.  This requirement comes from Kubernetes which does not update
config maps in `subPath` mounts.  The plugin expects the configuration
to be mounted to `/config` which in the end should contain a file
called `cex_resources.json`.

This file is periodically checked for updates.  Updates to this file
trigger reloads of the configuration which result in new plugin
instances being created for new configuration sets and plugin
instances for deleted configuration sets being removed.  Furthermore,
existing plugin instances will get the updated configuration map and
may announce new devices or remove devices.  Note that removing a
device from the configuration set does not terminate any container
currently using the device.

# Implementation

The implementation periodically checks the file
`/config/cex_resources.json` for updates.  To detect updates, it
computes a sha256 hash of the file.  If the hash changes, it will
reload the configuration.  Errors during reloading of the
configuration get logged, but the plugin continues with the old
configuration.

## Environment Support
The time interval between checks for configuration change can be
configured via the environment variable `CRYPTOCONFIG_CHECK_INTERVAL`.
It should be set to the delay between checks in seconds and defaults
to 120.  Note that the time interval has direct influence on the load
inside the plugin container.  It should be set such that the system
reacts in an appropriate time to configuration changes.

Default: 120
Unit: seconds
Range: >= 120
Range Violation: Set to 120
