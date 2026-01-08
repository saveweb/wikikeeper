#!/bin/bash
# Quick development environment setup script

set -e

echo "🚀 WikiKeeper Development Environment Setup"
echo "=========================================="
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

echo "✓ Docker is running"
echo ""

# Check if make is available
if ! command -v make &> /dev/null; then
    echo "❌ 'make' command not found. Please install make."
    exit 1
fi

echo "✓ Make is available"
echo ""

# Install development tools
echo "📦 Installing development tools..."
make install-tools
echo ""

# Start development database
echo "🗄️  Starting development database..."
make dev-up
echo ""

# Run migrations
echo "🔄 Running database migrations..."
make db-migrate
echo ""

# Install Go dependencies
echo "📦 Installing Go dependencies..."
make deps
echo ""

echo "✅ Development environment setup complete!"
echo ""
echo "Available commands:"
echo "  make run              - Run backend locally"
echo "  make run-dev          - Run backend with hot reload"
echo "  make test             - Run tests"
echo "  make db-shell         - Open PostgreSQL shell"
echo "  make dev-down         - Stop development environment"
echo "  make dev-tools        - Start database + Adminer (DB UI)"
echo ""
echo "Database connection:"
echo "  Host: localhost"
echo "  Port: 5432"
echo "  User: wikikeeper"
echo "  Password: wikikeeper123"
echo "  Database: wikikeeper"
echo ""
echo "To start developing:"
echo "  1. Run 'make run' or 'make run-dev'"
echo "  2. Backend will be available at http://localhost:8000"
echo ""
