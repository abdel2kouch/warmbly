#!/bin/sh
set -eu

# Remove the temporary sslip.io routers installed during initial setup.
rm -f /target/warmbly.yml /target/warmbly-mailpit.yml

# Keep only the permanent kouchgroup.com routes managed by this repository.
cp /source/warmbly-kouchgroup.yml /target/warmbly-kouchgroup.yml
