FROM python:3.11-slim

# Build arguments for metadata
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION

# Labels for image metadata
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="Arbiter Validator" \
      org.opencontainers.image.description="Configuration validation container for Arbiter Pipeline Infrastructure"

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Install AWS CLI v2
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf aws awscliv2.zip

# Install Python packages
RUN pip install --no-cache-dir \
    jsonschema \
    PyYAML

# Create working directory
WORKDIR /workspace

# Copy the aphex-validate script
COPY aphex-validate /usr/local/bin/aphex-validate
RUN chmod +x /usr/local/bin/aphex-validate

# Set environment variables
ENV PYTHONUNBUFFERED=1

# Default entrypoint
ENTRYPOINT ["/usr/local/bin/aphex-validate"]
