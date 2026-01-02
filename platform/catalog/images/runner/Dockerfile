FROM node:20-alpine

# Install system dependencies
RUN apk add --no-cache \
    git \
    curl \
    bash \
    python3 \
    py3-pip \
    ca-certificates

# Install Terraform
ARG TERRAFORM_VERSION=1.6.0
RUN wget https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    unzip terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    mv terraform /usr/local/bin/ && \
    rm terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    terraform version

# Install CDKTF CLI
RUN npm install -g cdktf-cli@latest && \
    cdktf --version

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/ && \
    kubectl version --client

# Set working directory
WORKDIR /workspace

# Verify all tools are installed
RUN echo "=== Installed Tools ===" && \
    node --version && \
    npm --version && \
    git --version && \
    terraform version && \
    cdktf --version && \
    kubectl version --client

# Default command
CMD ["/bin/bash"]
