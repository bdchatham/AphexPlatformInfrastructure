FROM node:20-alpine

# Build arguments for metadata
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION

# Labels for image metadata
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="Arbiter Deployer" \
      org.opencontainers.image.description="Deployment container for Arbiter Pipeline Infrastructure"

# Install system dependencies
RUN apk add --no-cache \
    python3 \
    py3-pip \
    git \
    curl \
    bash \
    gcc \
    g++ \
    make \
    libc-dev \
    linux-headers

# Install Python 3.11 (Alpine 3.18+ has Python 3.11)
RUN python3 --version

# Install AWS CLI v2
RUN apk add --no-cache \
    aws-cli

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

# Install AWS CDK CLI globally
RUN npm install -g aws-cdk

# Create working directory
WORKDIR /workspace

# Copy the deployment scripts
COPY aphex-deploy-pipeline /usr/local/bin/aphex-deploy-pipeline
COPY aphex-deploy-stack /usr/local/bin/aphex-deploy-stack
RUN chmod +x /usr/local/bin/aphex-deploy-pipeline && \
    chmod +x /usr/local/bin/aphex-deploy-stack

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV NODE_ENV=production

# Default entrypoint
ENTRYPOINT ["/bin/bash"]
