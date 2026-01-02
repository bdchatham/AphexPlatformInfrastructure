#!/bin/bash

# This script exposes Lighthouse webhooks for local testing
# Run this in a separate terminal and leave it running

echo "Exposing Lighthouse webhook service on localhost:8080..."
echo "GitHub webhook URL will be: http://localhost:8080/hook"
echo ""
echo "Press Ctrl+C to stop"
echo ""

kubectl port-forward -n pipeline-system service/hook 8080:80
