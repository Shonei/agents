#!/bin/bash
# Audit Viewer Startup Script

cd "$(dirname "$0")/audit-viewer"

# Check if bun is installed
if ! command -v bun &> /dev/null; then
    echo "❌ Bun is not installed. Please install it first:"
    echo "   curl -fsSL https://bun.sh/install | bash"
    exit 1
fi

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "📦 Installing dependencies..."
    bun install
fi

echo "🚀 Starting Audit Viewer..."
echo "   Open http://localhost:3000 in your browser"
echo ""
bun run dev
