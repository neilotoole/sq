#!/usr/bin/env python3
"""
Split a SARIF file with multiple runs into separate files.

GitHub Code Scanning allows up to 20 runs per SARIF file. Some tools (such as
Codacy) can produce SARIF files with more than 20 runs, which exceeds this
limit. This script splits a SARIF file with multiple runs into separate files,
each containing exactly one run to ensure compatibility with GitHub's limit.

Usage:
    python split-sarif.py <input-sarif-file> <output-dir>

Example:
    python split-sarif.py results.sarif ./sarif-output
"""

import json
import sys
import os
from pathlib import Path


def split_sarif(input_file, output_dir):
    """
    Split a SARIF file into multiple files, one run per file.

    Args:
        input_file: Path to the input SARIF file
        output_dir: Directory to write split SARIF files

    Returns:
        List of output file paths
    """
    # Read the input SARIF file
    with open(input_file, 'r', encoding='utf-8') as f:
        sarif_data = json.load(f)

    # Check if 'runs' exists
    if 'runs' not in sarif_data:
        print(f"No 'runs' found in {input_file}")
        return []

    runs = sarif_data['runs']
    total_runs = len(runs)

    print(f"Total runs found: {total_runs}")
    print("Creating one SARIF file per run...")

    # Create output directory if it doesn't exist
    Path(output_dir).mkdir(parents=True, exist_ok=True)

    output_files = []

    # Create one file per run
    for i, run in enumerate(runs, start=1):
        # Get tool name for better file naming
        tool_name = "unknown"
        if 'tool' in run and 'driver' in run['tool'] and 'name' in run['tool']['driver']:
            tool_name = run['tool']['driver']['name']
            # Sanitize tool name for filename
            tool_name = tool_name.replace('/', '-').replace('\\', '-').replace(' ', '-')

        # Add automationDetails.id for unique category
        # This is required by upload-sarif when uploading a directory of files
        # so each file has a distinct category
        run_id = f"codacy/{tool_name}/{i}"
        if 'automationDetails' not in run:
            run['automationDetails'] = {}
        run['automationDetails']['id'] = run_id

        # Create a new SARIF object with a single run
        single_run_sarif = {
            **sarif_data,  # Copy all top-level properties
            'runs': [run]  # Only one run per file
        }

        # Generate output filename with tool name and index
        output_file = os.path.join(output_dir, f'results-{i:02d}-{tool_name}.sarif')

        # Write the single run to file
        with open(output_file, 'w', encoding='utf-8') as out:
            json.dump(single_run_sarif, out, indent=2)

        output_files.append(output_file)
        print(f"  [{i}/{total_runs}] Created {os.path.basename(output_file)}")

    return output_files


def main():
    """Entry point for the script."""
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)

    input_file = sys.argv[1]
    output_dir = sys.argv[2]

    if not os.path.exists(input_file):
        print(f"Error: Input file '{input_file}' not found")
        sys.exit(1)

    output_files = split_sarif(input_file, output_dir)

    if output_files:
        print(f"\n✓ Successfully split {input_file} into {len(output_files)} file(s)")
    else:
        print("No output files created")
        sys.exit(1)


if __name__ == '__main__':
    main()
