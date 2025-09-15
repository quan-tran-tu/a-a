#!/usr/bin/env python3
"""
Script to collect all Go code files from current directory and subdirectories,
format them, and write to a single output file.
"""

import glob

def collect_go_files():
    """Recursively find all .go files in current directory and subdirectories."""
    go_files = []
    
    # Use glob to find all .go files recursively
    for go_file in glob.glob("**/*.go", recursive=True):
        go_files.append(go_file)
    
    # Sort files for consistent output
    return sorted(go_files)

def read_file_content(filepath):
    """Read content of a file, handling potential encoding issues."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return f.read()
    except Exception as e:
        return f"Error reading file: {e}"

def format_go_code_collection(output_file="go_code_collection.txt"):
    """Main function to collect and format all Go code."""
    go_files = collect_go_files()
    
    if not go_files:
        print("No Go files found in current directory and subdirectories.")
        return
    
    print(f"Found {len(go_files)} Go files:")
    for file in go_files:
        print(f"  - {file}")
    
    # Collect all content
    output_content = []
    
    for i, filepath in enumerate(go_files, 1):
        output_content.append(f"// File: {filepath}")
        
        # Read file content
        content = read_file_content(filepath)
        output_content.append(content)
        
        # Add separator between files (except for last file)
        if i < len(go_files):
            output_content.append("")  # Empty line separator
    
    # Join all content
    final_output = "\n".join(output_content)
    
    # Write to output file
    try:
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(final_output)
        print(f"\nSuccessfully wrote all Go code to: {output_file}")
        print(f"Total characters: {len(final_output)}")
    except Exception as e:
        print(f"Error writing to file: {e}")

if __name__ == "__main__":
    output_filename = "go_code_collection.txt"
    format_go_code_collection(output_filename)