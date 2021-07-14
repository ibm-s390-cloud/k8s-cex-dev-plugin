# Plugin Initialization

The shadow sysfs file system has to be initialized in the presence of
SELinux.  The essential problem is to create a folder that can be
accessed from any container.  To achieve this, use an initialization
container to create the folder and set the SELinux type to
`container_file_t`.

The current configuration creates the initialization container
`shadowsysfs` thats runs a `ubi-minimal` image to create the folder
`/var/tmp/shadowsysfs` on the shared volume mount `/var/tmp` with the
cex plugin container.  It then changes to SELinux context appropriately
and exits.  Because the container has to create the directory and
change the SELinux context, it has to be a privileged container.

## Debugging Initialization

To debug initialization, add a `sleep 3600` (or any other sufficient
time for debugging) to the command issued in the initialization
container and, for example, use `oc rsh -c shadowsysfs <pod>` to log
into the pod.  For production purposes, do not wait additional time
in the initialization container.

## Alternatives

Instead of using an initialization container, the cex plugin container
could be built on top of an base image, for example, ubi-minimal and
let the cex plugin set up the SELinux context.  Using an initialization
container, however, provides greater flexibility and easier
customization.
