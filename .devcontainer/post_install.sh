#!/bin/bash
set -ex

# Set up Python
rm -rf /home/vscode/.uv_cache | true
mkdir /home/vscode/.uv_cache | true
uv venv --clear

# Install pre-commit hooks
uv pip install -r requirements.txt
uv run pre-commit install

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
