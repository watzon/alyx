# Alyx Python Runtime Image
# This image provides the execution environment for Python functions

FROM python:3.11-alpine

# Set working directory
WORKDIR /runtime

# Create non-root user for security
RUN addgroup -g 1001 alyx && \
    adduser -D -u 1001 -G alyx alyx

# Create functions directory
RUN mkdir -p /functions && chown -R alyx:alyx /functions

# Copy runtime files
COPY requirements.txt ./
COPY executor.py ./
COPY alyx_functions/ ./alyx_functions/

# Install dependencies (none currently, but keeping the pattern)
RUN pip install --no-cache-dir -r requirements.txt

# Set ownership
RUN chown -R alyx:alyx /runtime

# Switch to non-root user
USER alyx

# Expose the executor port
EXPOSE 8080

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV PORT=8080
ENV FUNCTIONS_DIR=/functions

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Start the executor
CMD ["python", "executor.py"]
