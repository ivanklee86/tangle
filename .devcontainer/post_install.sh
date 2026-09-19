#!/bin/bash
set -ex

# Set up Python (mkdocs)
rm -rf /home/vscode/.uv_cache | true
mkdir /home/vscode/.uv_cache | true
uv venv --clear
uv pip install -r requirements.txt

# Configure git
if [ "$CODESPACES" != "true" ]; then
    echo "Running locally, configure gpg."

    git config --global gpg.program gpg2
    git config --global commit.gpgsign true
else
    echo "Running in codespaces."
fi

git config --global --add --bool push.autoSetupRemote true

# Woohoo!
echo "Hooray, it's done!"
