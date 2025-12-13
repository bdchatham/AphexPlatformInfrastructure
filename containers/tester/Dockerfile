FROM node:20-alpine

# Build arguments for metadata
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION

# Labels for image metadata
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="Arbiter Tester" \
      org.opencontainers.image.description="Test execution container for Arbiter Pipeline Infrastructure"

# Install system dependencies
RUN apk add --no-cache \
    python3 \
    py3-pip \
    bash \
    curl \
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

# Install Node.js testing frameworks globally
RUN npm install -g \
    jest \
    mocha

# Install Python testing framework
RUN pip3 install --no-cache-dir pytest

# Create working directory
WORKDIR /workspace

# Copy the aphex-test script
COPY aphex-test /usr/local/bin/aphex-test
RUN chmod +x /usr/local/bin/aphex-test

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV NODE_ENV=test

# Default entrypoint
ENTRYPOINT ["/usr/local/bin/aphex-test"]
