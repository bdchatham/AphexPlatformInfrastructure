FROM node:20-alpine

# Build arguments for metadata
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION

# Labels for image metadata
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="Arbiter Builder" \
      org.opencontainers.image.description="Build container for Arbiter Pipeline Infrastructure"

# Install system dependencies and build tools
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

# Create working directory
WORKDIR /workspace

# Copy the aphex-build script
COPY aphex-build /usr/local/bin/aphex-build
RUN chmod +x /usr/local/bin/aphex-build

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV NODE_ENV=production

# Default entrypoint
ENTRYPOINT ["/usr/local/bin/aphex-build"]
