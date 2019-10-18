#!/usr/bin/env bash
set -euo pipefail

echo "Installing cnab-azure-driver"
# Check if running in CloudShell

if [ -z ${ACC_CLOUD} ] 
then
    echo "Not running in Azure CloudShell"
    exit 1
fi

# check to see if porter is installed and if not install it
if [ ! -f ~/.porter/porter ] && [ -z $(which porter) ]
then
    curl https://cdn.deislabs.io/porter/latest/install-linux.sh|/bin/bash
    echo "" >> "${HOME}/.bashrc"
    echo "# Updating Path to include porter" >> "${HOME}/.bashrc"
    echo "export PATH=${HOME}/.porter:${PATH}" >> "${HOME}/.bashrc"
    echo "# Finish Updating Path to include porter" >> "${HOME}/.bashrc"
    echo "" >> "${HOME}/.bashrc"
    echo ".bashrc updated added porter dir to path"
    export PATH="${HOME}/.porter:${PATH}"
else
    echo "Porter Is Already Installed."
fi

# check to see if cnab-azure-driver is installed and if not install it

if [ ! -f ~/.cnab-azure-driver/cnab-azure-driver ] && [ -z $(which cnab-azure-driver) ]
then
    echo "Installing CNAB azure driver"
    DOWNLOAD_LOCATION=$( curl -sL https://api.github.com/repos/simongdavies/cnab-azure-driver/releases/latest | jq '.assets[]|select(.name=="cnab-azure-linux-amd64").browser_download_url' -r)
    mkdir -p ${HOME}/.cnab-azure-driver
    curl -sSLo ${HOME}/.cnab-azure-driver/cnab-azure "${DOWNLOAD_LOCATION}"
    chmod +x ${HOME}/.cnab-azure-driver/cnab-azure
    echo "" >> "${HOME}/.bashrc"
    echo "# Updating Path to include cnab-azure-driver" >> "${HOME}/.bashrc"
    echo "export PATH=${HOME}/.cnab-azure-driver:${PATH}" >> "${HOME}/.bashrc"
    echo "# Finish Updating Path to include cnab-azure-driver" >> "${HOME}/.bashrc"
    echo "" >> "${HOME}/.bashrc"
    echo ".bashrc updated added cnab-azure-driver dir to path"
    echo "Installed " $(${HOME}/.cnab-azure-driver/cnab-azure version)
else
    echo "CNAB Azure driver is already installed."
fi