#!/bin/bash

# Radio Contest Winner Docker Entrypoint Script
# Handles container startup and configuration

set -euo pipefail

# Default configuration
DEFAULT_CONFIG_FILE="/app/configs/config.yaml"
DEFAULT_OUTPUT_DIR="/app/output"
DEFAULT_MODELS_DIR="/app/models"

# Function to log messages (only in debug mode)
log() {
    if [ "${DEBUG_ENTRYPOINT:-}" = "true" ]; then
        echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
    fi
}

# Function to check if required directories exist
ensure_directories() {
    mkdir -p "$DEFAULT_OUTPUT_DIR" "$DEFAULT_MODELS_DIR"
    log "Ensured output and models directories exist"
}

# Function to check if Whisper model exists
check_whisper_model() {
    if [ ! -f "$DEFAULT_MODELS_DIR/ggml-base.en.bin" ]; then
        log "WARNING: Whisper model not found at $DEFAULT_MODELS_DIR/ggml-base.en.bin"
        log "Application will use API fallback if available"
    else
        log "Whisper model found at $DEFAULT_MODELS_DIR/ggml-base.en.bin"
    fi
}

# Function to validate configuration
validate_config() {
    if [ -f "$DEFAULT_CONFIG_FILE" ]; then
        log "Configuration file found at $DEFAULT_CONFIG_FILE"
    else
        log "No configuration file found, using defaults"
    fi
}

# Main entrypoint logic
main() {
    log "Radio Contest Winner container starting..."

    # Ensure required directories exist
    ensure_directories

    # Check Whisper model availability
    check_whisper_model

    # Validate configuration
    validate_config

    # Execute the main application with all passed arguments
    log "Starting Radio Contest Winner application..."
    exec /app/radiocontestwinner "$@"
}

# Run main function with all arguments
main "$@"