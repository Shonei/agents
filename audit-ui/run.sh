#!/bin/bash
# Start backend
bun server.ts &
SERVER_PID=$!

# Start frontend
bun dev &
FRONTEND_PID=$!

# Cleanup on exit
trap "kill $SERVER_PID $FRONTEND_PID" EXIT

wait
