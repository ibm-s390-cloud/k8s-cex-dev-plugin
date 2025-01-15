# Release Notes

## Version 1.0

Version 1.0 of the Kubernetes device plug-in for IBM Crypto Express (CEX) cards
is the initial release of this plug-in. It provides containerized applications
access to IBM Crypto Express (CEX) cards on IBM Z® and IBM LinuxONE (s390).

### Features

The following features are included in the initial release:

* Enable CEX cards for pods
* Configure available crypto sets by using a ConfigMap
* Static copy of the sysfs for the pod
* Overcommitment of CEX Resources
* Hot plug and hot unplug of APQNS

## Version 1.0.2

Version 1.0.2 of the Kubernetes device plug-in for IBM Crypto Express (CEX)
cards contains one new feature and one bug fix.

### Features

This update includes one new feature:

* The OVERCOMMIT limit can be specified per configset with an `overcommit:`
  field.

### Resolved issues

* The code has been rebuilt with updated libraries because of a CVE finding in
  the protobuf package:
   * [https://nvd.nist.gov/vuln/detail/CVE-2021-3121](https://nvd.nist.gov/vuln/detail/CVE-2021-3121)
   * [http://github.com/gogo/protobuf](http://github.com/gogo/protobuf).

## Version 1.1.0

The new version 1.1.0 of the Kubernetes device plug-in for IBM Crypto Express
(CEX) cards includes the exploration of Prometheus metrics around the crypto
resources managed by the plug-in. It also comes with support for *OCP 4.12* with
enforced *Security Context Constrains (SCC)*. By default the CEX device plugin
now installes into it's own namespace *cex-device-plugin* and all the
deployments have been rearranged for *Kustomize* support to make it easier to
create and update the CEX device plugin entities.

### Features

- Prometheus metrics support
- New namespace 'cex-device-plugin' (See
  [Migrating from kube-system to cex-device-plugin Namespace](migration.md))
- Enabled for OCP 4.12 with enforced SCC support
- Kustimize support

### Resolved issues

The code has been rebuild with updated libraries because of CVE findings in
libraries the CEX device plug-in depends on:
- CVE-2022-3172 (Medium) in k8s.io/apiMachinery-v0.20.4
- CVE-2022-21698 (High) in github.com/prometheus/client_goLang-v1.11.0
- CVE-2022-32149 (High) in golang.org/x/text-v0.3.4

## Version 1.1.1

The code has been rebuilt with updated libraries because of a CVE finding
in a dependent library:
- CVE-2023-39325 (High) in golang.org/x/net-v0.9.0 (see [https://nvd.nist.gov/vuln/detail/CVE-2023-39325](https://nvd.nist.gov/vuln/detail/CVE-2023-39325))

## Version 1.1.2

The code has been rebuilt with upgrade of the go language to 1.21 and updated
libraries because of minor complains by quay.io and github at dependent libraries.

## Version 1.2.0

Version 1.2.0 is a minor release with one new feature: live-sysfs support (see
below). Also the go build has been updated to go language version 1.22 and some
of the depentent libraries where updated because of CVE findings
(golang.org/x/net update to v0.33.0, grpc update to 1.64.1).

### Features

Live sysfs support within the shadow sysfs: With *live sysfs support* enabled
some of the files shadowed to the directories `/sys/bus/ap` and
`/sys/devices/ap` become alive. For example `online` attribute reflects the real
state of this APQN of the providing host node. More details can be found in the
CEX-plugin documentation.

## Known issues

There are no known issues. See
[Limitations](technical_concepts_limitations.md#limitations) for the list of
current limitations.

<!--For a list of the features and improvements that were introduced in version xx , see What's new
With this offering, the following new features are introduced:
These release notes contain:
    New features summary
    Known issues
    Resolved issues
    Prerequisites and installation information
# Resolved issues
The release includes various bug fixes.-->
