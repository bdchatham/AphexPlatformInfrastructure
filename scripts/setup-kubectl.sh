#!/bin/bash
set -e

# Setup kubectl access to the Arbiter EKS cluster
# This script configures kubectl to use the appropriate IAM role for cluster access

STACK_NAME="ArbiterPipelineInfrastructureStack"
REGION="${AWS_REGION:-us-east-1}"

# Check if caller is root (not allowed)
CALLER_IDENTITY=$(aws sts get-caller-identity --query 'Arn' --output text)
if [[ "$CALLER_IDENTITY" == *":root" ]]; then
  echo "❌ Error: Root credentials detected"
  echo "   Root cannot assume IAM roles and should never be used for kubectl access."
  echo "   Please use an IAM user or role instead."
  exit 1
fi

echo "✓ Using identity: $CALLER_IDENTITY"
echo ""

# Get stack outputs
echo "📡 Fetching cluster information from CloudFormation..."
CLUSTER_NAME=$(aws cloudformation describe-stacks \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --query 'Stacks[0].Outputs[?contains(OutputKey, `ClusterNameOutput`)].OutputValue' \
  --output text)

READ_ONLY_ROLE_ARN=$(aws cloudformation describe-stacks \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --query 'Stacks[0].Outputs[?contains(OutputKey, `ReadOnlyRoleArnOutput`)].OutputValue' \
  --output text)

BREAKGLASS_ROLE_ARN=$(aws cloudformation describe-stacks \
  --stack-name "$STACK_NAME" \
  --region "$REGION" \
  --query 'Stacks[0].Outputs[?contains(OutputKey, `BreakglassAdminRoleArnOutput`)].OutputValue' \
  --output text)

if [ -z "$CLUSTER_NAME" ] || [ -z "$READ_ONLY_ROLE_ARN" ]; then
  echo "❌ Error: Could not fetch cluster information from stack"
  echo "   Make sure the stack is deployed and you have permissions to describe it."
  exit 1
fi

echo "✓ Found cluster: $CLUSTER_NAME"
echo ""

# Determine which role to use
MODE="${1:-read-only}"

if [ "$MODE" == "breakglass" ] || [ "$MODE" == "admin" ]; then
  ROLE_ARN="$BREAKGLASS_ROLE_ARN"
  echo "⚠️  Configuring BREAKGLASS ADMIN access"
  echo "   This grants full cluster admin privileges."
  echo "   Use only for emergency operations."
else
  ROLE_ARN="$READ_ONLY_ROLE_ARN"
  echo "✓ Configuring READ-ONLY access (default)"
  echo "   This grants view-only permissions for introspection."
fi

echo ""
echo "Role ARN: $ROLE_ARN"
echo ""

# Update kubeconfig
echo "🔧 Updating kubeconfig..."
aws eks update-kubeconfig \
  --name "$CLUSTER_NAME" \
  --region "$REGION" \
  --role-arn "$ROLE_ARN"

echo ""
echo "✅ kubectl configured successfully!"
echo ""
echo "Test your access:"
echo "  kubectl get namespaces"
echo "  kubectl auth can-i create deployment -n default"
echo ""

if [ "$MODE" == "read-only" ]; then
  echo "To switch to breakglass admin access (emergency only):"
  echo "  $0 breakglass"
  echo ""
fi
