# Release Notes

## Version 1.0

Version 1.0 of the Kubernetes device plug-in for IBM Crypto Express (CEX) cards is the initial release of this plug-in.
It provides containerized applications access to IBM Crypto Express (CEX) cards on IBM Z® and IBM LinuxONE (s390).

### Features

The following features are included in the initial release:

* Enable CEX cards for pods
* Configure available crypto sets by using a ConfigMap
* Static copy of the sysfs for the pod
* Overcommitment of CEX Resources
* Hot plug and hot unplug of APQNS

## Version 1.0.2

Version 1.0.2 of the Kubernetes device plug-in for IBM Crypto Express (CEX) cards contains one new feature and one bug fix.

### Features

This update includes one new feature:

* The OVERCOMMIT limit can be specified per configset with an `overcommit:` field.

### Resolved issues

* The code has been rebuilt with updated libraries because of a CVE finding in the protobuf package.
   * [https://nvd.nist.gov/vuln/detail/CVE-2021-3121](https://nvd.nist.gov/vuln/detail/CVE-2021-3121)
   * [http://github.com/gogo/protobuf](http://github.com/gogo/protobuf).

## Known issues

There are no known issues. See [Limitations](technical_concepts_limitations.md#limitations) for the list of current limitations.

<!--For a list of the features and improvements that were introduced in version xx , see What's new
With this offering, the following new features are introduced:
These release notes contain:
    New features summary
    Known issues
    Resolved issues
    Prerequisites and installation information
# Resolved issues
The release includes various bug fixes.-->
