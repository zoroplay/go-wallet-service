#!/bin/bash
set -e

# ==== EDIT THESE TWO LINES ONLY FOR EACH RUNNER ====
RUNNER_NAME="go-wallet-service"
REPO_URL="https://github.com/zoroplay/go-wallet-service"
TOKEN="AYOWC7SHQV57SKPHNT6UETTJLQWBS"
RUNNER_DIR=~/${RUNNER_NAME}
VERSION="2.330.0"
HASH="af5c33fa94f3cc33b8e97937939136a6b04197e6dadfcfb3b6e33ae1bf41e79a"

echo "Setting up $RUNNER_NAME ..."

# Clean any old traces
sudo systemctl stop actions.runner.*${RUNNER_NAME}* 2>/dev/null || true
sudo rm -rf "$RUNNER_DIR" /etc/systemd/system/actions.runner.*${RUNNER_NAME}*

# Fresh install
mkdir -p "$RUNNER_DIR" && cd "$RUNNER_DIR"
curl -o runner.tar.gz -L https://github.com/actions/runner/releases/download/v$VERSION/actions-runner-linux-x64-$VERSION.tar.gz
echo "$HASH  runner.tar.gz" | shasum -a 256 -c
tar xzf runner.tar.gz && rm runner.tar.gz

# Register (non-interactive)
./config.sh --url "$REPO_URL" --token "$TOKEN" --name "$(hostname)-$RUNNER_NAME" --labels "ubuntu,production" --work _work --unattended --replace

# Install & start service
sudo ./svc.sh install
sudo ./svc.sh start

echo "$RUNNER_NAME is done! Status:"
sudo ./svc.sh status | grep -E "Active|Online"
echo "Go to GitHub -> Settings -> Actions -> Runners to see it online in a few seconds."
echo "-------------------------------------------------------------"