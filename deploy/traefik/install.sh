#!/bin/sh
set -eu

# Remove the temporary sslip.io routers installed during initial setup. The
# glob also catches an older .yaml spelling used by the protected Mailpit route.
for config in /target/warmbly*; do
    [ -f "$config" ] || continue
    if grep -q 'compose-back-up-mobile-system-eu5pcz-d70d6f-72-60-81-29\.sslip\.io' "$config"; then
        rm -f "$config"
    fi
done

# Keep only the permanent kouchgroup.com routes managed by this repository.
cp /source/warmbly-kouchgroup.yml /target/warmbly-kouchgroup.yml
