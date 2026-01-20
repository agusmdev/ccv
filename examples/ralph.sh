#!/bin/bash
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <iterations> [model]"
  echo "  model: sonnet or opus (default: opus)"
  exit 1
fi

MODEL="${2:-opus}"
if [ "$MODEL" != "sonnet" ] && [ "$MODEL" != "opus" ]; then
  echo "Error: model must be 'sonnet' or 'opus'"
  exit 1
fi

# Temp file for capturing output to check for <no-tasks/>
temp_output=$(mktemp)
trap "rm -f $temp_output" EXIT

for ((i=1; i<=$1; i++)); do
  echo ""
  echo "========================================"
  echo "  ITERATION $i"
  echo "========================================"
  echo ""

  # Run ccv which handles all output formatting
  # Use tee to both display output and capture it for checking
  ./ccv --model "$MODEL" --dangerously-skip-permissions -p \
    "Use bd (beads) to find and work on a task:
1. Run 'bd ready' to find an available task
2. If no tasks available, output <no-tasks/> and stop
3. Claim the task with 'bd update <id> --status in_progress'
4. Implement the task according to its acceptance criteria
5. Make a git commit with your changes
6. Close the task with 'bd close <id>'
ONLY WORK ON A SINGLE TASK." 2>&1 | tee "$temp_output"

  # Check if output contains no-tasks marker
  if grep -q "<no-tasks/>" "$temp_output" 2>/dev/null; then
    echo ""
    echo "========================================"
    echo "  ALL TASKS COMPLETE - Exiting"
    echo "========================================"
    exit 0
  fi

  # Clear temp file for next iteration
  > "$temp_output"
done

echo ""
echo "========================================"
echo "  All $1 iterations completed"
echo "========================================"
