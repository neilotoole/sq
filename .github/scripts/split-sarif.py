#!/usr/bin/env python3
"""
Split a SARIF file with multiple runs into separate files.

This script splits a SARIF file that contains more than 20 runs (GitHub's limit)
into multiple SARIF files, each containing a maximum of 20 runs.

Usage:
    python split-sarif.py <input-sarif-file> <output-dir> [max-runs-per-file]

Example:
    python split-sarif.py results.sarif ./sarif-output 20
"""

import json
import sys
import os
from pathlib import Path


def split_sarif(input_file, output_dir, max_runs=20):
    """
    Split a SARIF file into multiple files based on the number of runs.

    Args:
        input_file: Path to the input SARIF file
        output_dir: Directory to write split SARIF files
        max_runs: Maximum number of runs per output file (default: 20)

    Returns:
        List of output file paths
    """
    # Read the input SARIF file
    with open(input_file, 'r') as f:
        sarif_data = json.load(f)

    # Check if 'runs' exists
    if 'runs' not in sarif_data:
        print(f"No 'runs' found in {input_file}")
        return []

    runs = sarif_data['runs']
    total_runs = len(runs)

    print(f"Total runs found: {total_runs}")
    print(f"Max runs per file: {max_runs}")

    # Create output directory if it doesn't exist
    Path(output_dir).mkdir(parents=True, exist_ok=True)

    output_files = []

    # Split runs into chunks
    for i in range(0, total_runs, max_runs):
        chunk_runs = runs[i:i + max_runs]
        chunk_number = (i // max_runs) + 1

        # Create a new SARIF object with the chunk of runs
        chunk_sarif = {
            **sarif_data,  # Copy all top-level properties
            'runs': chunk_runs
        }

        # Generate output filename
        output_file = os.path.join(output_dir, f'results-{chunk_number}.sarif')

        # Write the chunk to file
        with open(output_file, 'w') as f:
            json.dump(chunk_sarif, f, indent=2)

        output_files.append(output_file)
        print(f"Created {output_file} with {len(chunk_runs)} runs")

    return output_files


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)

    input_file = sys.argv[1]
    output_dir = sys.argv[2]
    max_runs = int(sys.argv[3]) if len(sys.argv) > 3 else 20

    if not os.path.exists(input_file):
        print(f"Error: Input file '{input_file}' not found")
        sys.exit(1)

    output_files = split_sarif(input_file, output_dir, max_runs)

    if output_files:
        print(f"\nSuccessfully split {input_file} into {len(output_files)} file(s)")
        print(f"Output files: {', '.join(output_files)}")
    else:
        print("No output files created")
        sys.exit(1)


if __name__ == '__main__':
    main()
