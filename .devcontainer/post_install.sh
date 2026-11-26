#!/bin/bash
set -ex

# Set up Python (mkdocs)
rm -rf /home/vscode/.uv_cache | true
mkdir /home/vscode/.uv_cache | true
uv venv --clear
uv pip install -r requirements.txt

# Install web dependencies exactly as locked, so the Playwright package matches
# the browsers the Dockerfile installs (both pinned to the same version).
npm ci --prefix web

# Configure git
git config --global --add --bool push.autoSetupRemote true

# Woohoo!
echo "Hooray, it's done!"
