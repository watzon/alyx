# Alyx Node.js Runtime Image
# This image provides the execution environment for Node.js functions

FROM node:23-alpine

# Set working directory
WORKDIR /runtime

# Create non-root user for security
RUN addgroup -g 1001 alyx && \
    adduser -D -u 1001 -G alyx alyx

# Create functions directory
RUN mkdir -p /functions && chown -R alyx:alyx /functions

# Copy runtime files
COPY package.json ./
COPY executor.js ./
COPY sdk/ ./sdk/

# No npm install needed since we have no dependencies

# Set ownership
RUN chown -R alyx:alyx /runtime

# Switch to non-root user
USER alyx

# Expose the executor port
EXPOSE 8080

# Set environment variables
ENV NODE_ENV=production
ENV PORT=8080
ENV FUNCTIONS_DIR=/functions

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Start the executor
CMD ["node", "--experimental-transform-types", "executor.js"]
